import { useEffect, useRef, type ReactNode } from "react";
import "./Dialog.css";

type DialogProps = {
  title: string;
  onClose: () => void;
  children: ReactNode;
};

// A modal dialog. The native element traps the focus, closes on Escape and
// dims the page behind it.
function Dialog({ title, onClose, children }: DialogProps) {
  const ref = useRef<HTMLDialogElement | null>(null);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    dialog.showModal();
    return () => dialog.close();
  }, []);

  return (
    <dialog
      ref={ref}
      className="dialog"
      aria-label={title}
      // Escape. The "close" event is not used: it also fires when the
      // element is closed on unmount, when there is nothing left to close.
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onClick={(event) => {
        // The dialog element itself is only hit on the backdrop.
        if (event.target === ref.current) onClose();
      }}
    >
      <div className="dialogHeader">
        <h2>{title}</h2>
        <button type="button" className="dialogClose" onClick={onClose} aria-label="閉じる">
          ×
        </button>
      </div>
      <div className="dialogBody">{children}</div>
    </dialog>
  );
}

export default Dialog;
