package workspace

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/loft-sh/devpod/cmd/flags"
	"github.com/loft-sh/devpod/pkg/agent"
	"github.com/loft-sh/devpod/pkg/agent/tunnel"
	"github.com/loft-sh/devpod/pkg/command"
	"github.com/loft-sh/devpod/pkg/credentials"
	"github.com/loft-sh/devpod/pkg/daemon"
	"github.com/loft-sh/devpod/pkg/devcontainer"
	config2 "github.com/loft-sh/devpod/pkg/devcontainer/config"
	"github.com/loft-sh/devpod/pkg/dockercredentials"
	"github.com/loft-sh/devpod/pkg/extract"
	"github.com/loft-sh/devpod/pkg/gitcredentials"
	"github.com/loft-sh/devpod/pkg/log"
	"github.com/loft-sh/devpod/pkg/port"
	provider2 "github.com/loft-sh/devpod/pkg/provider"
	"github.com/loft-sh/devpod/pkg/random"
	"github.com/loft-sh/devpod/scripts"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// UpCmd holds the up cmd flags
type UpCmd struct {
	*flags.GlobalFlags

	WorkspaceInfo        string
	PrebuildRepositories []string

	ForceBuild bool
	Recreate   bool
}

// NewUpCmd creates a new command
func NewUpCmd(flags *flags.GlobalFlags) *cobra.Command {
	cmd := &UpCmd{
		GlobalFlags: flags,
	}
	upCmd := &cobra.Command{
		Use:   "up",
		Short: "Starts a new devcontainer",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return cmd.Run(context.Background())
		},
	}
	upCmd.Flags().BoolVar(&cmd.ForceBuild, "force-build", false, "If true will rebuild the container even if there is a prebuild already")
	upCmd.Flags().BoolVar(&cmd.Recreate, "recreate", false, "If true will remove any existing containers and recreate them")
	upCmd.Flags().StringSliceVar(&cmd.PrebuildRepositories, "prebuild-repository", []string{}, "Docker respository that hosts devpod prebuilds for this workspace")
	upCmd.Flags().StringVar(&cmd.WorkspaceInfo, "workspace-info", "", "The workspace info")
	_ = upCmd.MarkFlagRequired("workspace-info")
	return upCmd
}

// Run runs the command logic
func (cmd *UpCmd) Run(ctx context.Context) error {
	// get workspace
	shouldExit, workspaceInfo, err := agent.WriteWorkspaceInfo(cmd.WorkspaceInfo)
	if err != nil {
		return fmt.Errorf("error parsing workspace info: %v", err)
	} else if shouldExit {
		return nil
	}

	// initialize the workspace
	tunnelClient, logger, err := initWorkspace(ctx, workspaceInfo, cmd.Debug, true)
	if err != nil {
		return err
	}

	// get docker credentials
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	dir, err := configureDockerCredentials(cancelCtx, cancel, workspaceInfo, tunnelClient, logger)
	if err != nil {
		logger.Errorf("Error retrieving docker credentials: %v", err)
	} else if dir != "" {
		defer os.RemoveAll(dir)
	}

	// start up
	err = cmd.up(ctx, workspaceInfo, tunnelClient, logger)
	if err != nil {
		return errors.Wrap(err, "devcontainer up")
	}

	return nil
}

func initWorkspace(ctx context.Context, workspaceInfo *provider2.AgentWorkspaceInfo, debug, shouldInstallDaemon bool) (tunnel.TunnelClient, log.Logger, error) {
	// create a grpc client
	tunnelClient, err := agent.NewTunnelClient(os.Stdin, os.Stdout, true)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating tunnel client: %v", err)
	}

	// create debug logger
	logger := agent.NewTunnelLogger(ctx, tunnelClient, debug)

	// this message serves as a ping to the client
	_, err = tunnelClient.Ping(ctx, &tunnel.Empty{})
	if err != nil {
		return nil, nil, errors.Wrap(err, "ping client")
	}

	// install docker in background
	errChan := make(chan error)
	go func() {
		errChan <- InstallDocker(logger)
	}()

	// prepare workspace
	err = prepareWorkspace(ctx, workspaceInfo, tunnelClient, logger)
	if err != nil {
		return nil, nil, err
	}

	// install daemon
	if shouldInstallDaemon {
		err = installDaemon(workspaceInfo, logger)
		if err != nil {
			logger.Errorf("Install DevPod Daemon: %v", err)
		}
	}

	// wait until docker is installed
	err = <-errChan
	if err != nil {
		return nil, nil, errors.Wrap(err, "install docker")
	}

	return tunnelClient, logger, nil
}

func (cmd *UpCmd) up(ctx context.Context, workspaceInfo *provider2.AgentWorkspaceInfo, tunnelClient tunnel.TunnelClient, logger log.Logger) error {
	// create devcontainer
	result, err := cmd.devPodUp(workspaceInfo, logger)
	if err != nil {
		return err
	}

	// send result
	out, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = tunnelClient.SendResult(ctx, &tunnel.Message{Message: string(out)})
	if err != nil {
		return errors.Wrap(err, "send result")
	}

	return nil
}

func prepareWorkspace(ctx context.Context, workspaceInfo *provider2.AgentWorkspaceInfo, client tunnel.TunnelClient, log log.Logger) error {
	_, err := os.Stat(workspaceInfo.Folder)
	if err == nil {
		log.Debugf("Workspace Folder already exists")
		return nil
	}

	// make content dir
	err = os.MkdirAll(workspaceInfo.Folder, 0777)
	if err != nil {
		return errors.Wrap(err, "make workspace folder")
	}

	// check what type of workspace this is
	if workspaceInfo.Workspace.Source.GitRepository != "" {
		log.Debugf("Clone Repository")
		helper := ""
		if workspaceInfo.Agent.InjectGitCredentials == "true" {
			log.Debugf("Start credentials server")
			cancelCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			helper, err = startGitCredentialsHelper(cancelCtx, cancel, client, log)
			if err != nil {
				return err
			}
		}

		return CloneRepository(workspaceInfo.Folder, workspaceInfo.Workspace.Source.GitRepository, helper, log)
	} else if workspaceInfo.Workspace.Source.LocalFolder != "" {
		log.Debugf("Download Local Folder")
		return DownloadLocalFolder(ctx, workspaceInfo.Folder, client, log)
	} else if workspaceInfo.Workspace.Source.Image != "" {
		log.Debugf("Prepare Image")
		return PrepareImage(workspaceInfo.Folder, workspaceInfo.Workspace.Source.Image)
	}

	return fmt.Errorf("either workspace repository, image or local-folder is required")
}

func configureDockerCredentials(ctx context.Context, cancel context.CancelFunc, workspaceInfo *provider2.AgentWorkspaceInfo, client tunnel.TunnelClient, log log.Logger) (string, error) {
	if workspaceInfo.Agent.InjectDockerCredentials != "true" {
		return "", nil
	}

	serverPort, err := startCredentialsServer(ctx, cancel, client, log)
	if err != nil {
		return "", err
	}

	if workspaceInfo.Folder == "" {
		return "", fmt.Errorf("workspace folder is not set")
	}

	return dockercredentials.ConfigureCredentialsMachine(filepath.Join(workspaceInfo.Folder, ".."), serverPort)
}

func startCredentialsServer(ctx context.Context, cancel context.CancelFunc, client tunnel.TunnelClient, log log.Logger) (int, error) {
	port, err := port.FindAvailablePort(random.InRange(13000, 17000))
	if err != nil {
		return 0, err
	}

	go func() {
		defer cancel()

		err := credentials.RunCredentialsServer(ctx, "", port, false, false, client, log)
		if err != nil {
			log.Errorf("Run git credentials server: %v", err)
		}
	}()

	// wait until credentials server is up
	maxWait := time.Second * 4
	now := time.Now()
Outer:
	for {
		err := PingURL(ctx, "http://localhost:"+strconv.Itoa(port))
		if err != nil {
			select {
			case <-ctx.Done():
				break Outer
			case <-time.After(time.Second):
			}
		} else {
			log.Debugf("Credentials server started...")
			break
		}

		if time.Since(now) > maxWait {
			log.Debugf("Credentials server didn't start in time...")
			break
		}
	}

	return port, nil
}

func startGitCredentialsHelper(ctx context.Context, cancel context.CancelFunc, client tunnel.TunnelClient, log log.Logger) (string, error) {
	port, err := startCredentialsServer(ctx, cancel, client, log)
	if err != nil {
		return "", err
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s agent git-credentials --port %d", binaryPath, port), nil
}

func PingURL(ctx context.Context, url string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, "GET", url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

func installDaemon(workspaceInfo *provider2.AgentWorkspaceInfo, log log.Logger) error {
	if len(workspaceInfo.Agent.Exec.Shutdown) == 0 {
		return nil
	}

	log.Debugf("Installing DevPod daemon into server...")
	err := daemon.InstallDaemon(log)
	if err != nil {
		return errors.Wrap(err, "install daemon")
	}

	return nil
}

func DownloadLocalFolder(ctx context.Context, workspaceDir string, client tunnel.TunnelClient, log log.Logger) error {
	log.Infof("Upload folder to server")
	stream, err := client.ReadWorkspace(ctx, &tunnel.Empty{})
	if err != nil {
		return errors.Wrap(err, "read workspace")
	}

	err = extract.Extract(agent.NewStreamReader(stream), workspaceDir)
	if err != nil {
		return errors.Wrap(err, "extract local folder")
	}

	return nil
}

func PrepareImage(workspaceDir, image string) error {
	// create a .devcontainer.json with the image
	err := os.WriteFile(filepath.Join(workspaceDir, ".devcontainer.json"), []byte(`{
  "image": "`+image+`"
}`), 0666)
	if err != nil {
		return err
	}

	return nil
}

func (cmd *UpCmd) devPodUp(workspaceInfo *provider2.AgentWorkspaceInfo, log log.Logger) (*config2.Result, error) {
	result, err := createRunner(workspaceInfo, log).Up(devcontainer.UpOptions{
		PrebuildRepositories: cmd.PrebuildRepositories,

		ForceBuild: cmd.ForceBuild,
		Recreate:   cmd.Recreate,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func CloneRepository(workspaceDir, repository, helper string, log log.Logger) error {
	// run git command
	writer := log.Writer(logrus.InfoLevel, false)
	defer writer.Close()

	args := []string{"clone"}
	if helper != "" {
		args = append(args, "--config", "credential.helper="+helper)
	}
	args = append(args, repository, workspaceDir)
	gitCommand := exec.Command("git", args...)
	gitCommand.Stdout = writer
	gitCommand.Stderr = writer
	err := gitCommand.Run()
	if err != nil {
		return errors.Wrap(err, "error cloning repository")
	}

	// remove the credential helper or otherwise we will receive strange errors within the container
	if helper != "" {
		err = gitcredentials.RemoveHelperFromPath(filepath.Join(workspaceDir, ".git", "config"))
		if err != nil {
			return err
		}
	}

	return nil
}

func InstallDocker(log log.Logger) error {
	if !command.Exists("docker") {
		writer := log.Writer(logrus.InfoLevel, false)
		defer writer.Close()

		shellCommand := exec.Command("sh", "-c", scripts.InstallDocker)
		shellCommand.Stdout = writer
		shellCommand.Stderr = writer
		err := shellCommand.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func createRunner(workspaceInfo *provider2.AgentWorkspaceInfo, log log.Logger) *devcontainer.Runner {
	return devcontainer.NewRunner(agent.RemoteDevPodHelperLocation, agent.DefaultAgentDownloadURL, workspaceInfo, log)
}