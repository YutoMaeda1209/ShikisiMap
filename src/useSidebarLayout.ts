import { useCallback, useLayoutEffect, useRef, useState } from "react";

function useSidebarLayout() {
  const sidebarRef = useRef<HTMLDivElement | null>(null);
  const titleRef = useRef<HTMLDivElement | null>(null);
  const [listHeight, setListHeight] = useState(0);

  const updateListHeight = useCallback(() => {
    if (!sidebarRef.current || !titleRef.current) return;
    const nextHeight = Math.max(
      0,
      Math.floor(
        sidebarRef.current.clientHeight - titleRef.current.clientHeight - 50,
      ),
    );
    setListHeight(nextHeight);
  }, []);

  useLayoutEffect(() => {
    updateListHeight();
    const observer = new ResizeObserver(updateListHeight);
    if (sidebarRef.current) observer.observe(sidebarRef.current);
    if (titleRef.current) observer.observe(titleRef.current);
    window.addEventListener("resize", updateListHeight);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updateListHeight);
    };
  }, [updateListHeight]);

  return { sidebarRef, titleRef, listHeight };
}

export default useSidebarLayout;
