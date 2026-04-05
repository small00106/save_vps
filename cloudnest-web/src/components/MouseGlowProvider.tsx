import { useMouseGlow } from "../hooks/useMouseGlow";
import type { ReactNode } from "react";

interface MouseGlowProviderProps {
  children: ReactNode;
  /** Whether to show the glow layer. Disabled on mobile by default via CSS. */
  enabled?: boolean;
}

/**
 * Provider component that adds mouse-following glow effect to the page.
 * Wrap your app or specific sections with this component.
 */
export function MouseGlowProvider({
  children,
  enabled = true,
}: MouseGlowProviderProps) {
  useMouseGlow();

  return (
    <>
      {enabled && (
        <div
          className="mouse-glow-layer hidden md:block"
          aria-hidden="true"
        />
      )}
      {children}
    </>
  );
}

export default MouseGlowProvider;
