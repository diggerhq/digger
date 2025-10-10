import * as React from "react"

const MOBILE_BREAKPOINT = 768

const useIsomorphicLayoutEffect =
  typeof window !== "undefined" ? React.useLayoutEffect : React.useEffect

export function useIsMobile() {
  const get = () =>
    typeof window !== "undefined" &&
    window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`).matches

  const [isMobile, setIsMobile] = React.useState<boolean>(get)

  useIsomorphicLayoutEffect(() => {
    const mql = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`)
    const onChange = () => setIsMobile(mql.matches)
    setIsMobile(mql.matches)
    mql.addEventListener("change", onChange)
    return () => mql.removeEventListener("change", onChange)
  }, [])

  return isMobile
}
