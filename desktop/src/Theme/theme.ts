// We need the `.cts` filename for chakra-cli to properly generate our types
import { extendTheme, Theme, ThemeOverride } from "@chakra-ui/react"

export const theme = extendTheme({
  styles: {
    global() {
      return {
        html: {
          fontSize: "14px",
          overflow: "hidden",
          backgroun: "transparent",
        },
        body: {
          background: "transparent",
          userSelect: "none",
        },
        td: {
          userSelect: "auto",
        },
        code: {
          userSelect: "auto",
        },
      }
    },
  },
  colors: {
    primary: {
      400: "#AA40EE",
      500: "#BA50FF",
      600: "#CA60FF", // TODO: change to #CA60FF
    },
  },
  config: {
    initialColorMode: "system",
    useSystemColorMode: true,
  },
} satisfies ThemeOverride) as Theme
