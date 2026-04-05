import { useEffect, useCallback, useRef } from "react";

/**
 * Hook to track mouse position and update CSS variables for glow effects
 * Updates --mouse-x and --mouse-y on the document root
 */
export function useMouseGlow() {
  const rafRef = useRef<number | undefined>(undefined);
  const mouseRef = useRef({ x: 0, y: 0 });

  const updatePosition = useCallback(() => {
    const root = document.documentElement;
    root.style.setProperty("--mouse-x", `${mouseRef.current.x}px`);
    root.style.setProperty("--mouse-y", `${mouseRef.current.y}px`);
  }, []);

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      mouseRef.current = { x: e.clientX, y: e.clientY };

      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
      }
      rafRef.current = requestAnimationFrame(updatePosition);
    };

    window.addEventListener("mousemove", handleMouseMove, { passive: true });

    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
      }
    };
  }, [updatePosition]);
}

/**
 * Hook for card-level mouse glow effect
 * Returns handlers to attach to card elements
 */
export function useCardGlow() {
  const handleMouseMove = useCallback(
    (e: React.MouseEvent<HTMLElement>) => {
      const rect = e.currentTarget.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      e.currentTarget.style.setProperty("--local-x", `${x}px`);
      e.currentTarget.style.setProperty("--local-y", `${y}px`);
    },
    []
  );

  return { onMouseMove: handleMouseMove };
}
