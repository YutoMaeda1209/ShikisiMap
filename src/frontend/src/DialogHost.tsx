import { useEffect, useState } from "react";
import AccountDialog, { LocalImportSection } from "./AccountDialog";
import { useAuth } from "./authContext";
import Dialog from "./Dialog";
import { useMapData } from "./mapDataContext";
import VisitDialog from "./VisitDialog";
import { useVisits } from "./visitsContext";

type OpenDialog =
  | { kind: "visit"; spotId: string }
  | { kind: "account" }
  | { kind: "notice"; message: string }
  | null;

// Remembers for this tab that the visitor put the import off, so that the
// question is not repeated on every reload.
const importDismissedKey = "shikisimap.importDismissed";

function wasImportDismissed(): boolean {
  try {
    return window.sessionStorage.getItem(importDismissedKey) === "1";
  } catch {
    return false;
  }
}

// Shows the dialogs that other components ask for through window events,
// and the question to import local records after signing in.
function DialogHost() {
  const { me } = useAuth();
  const { allSpots } = useMapData();
  const { localImportCount } = useVisits();
  const [open, setOpen] = useState<OpenDialog>(null);
  const [importDismissed, setImportDismissed] = useState(wasImportDismissed);

  useEffect(() => {
    function openVisit(event: Event) {
      setOpen({ kind: "visit", spotId: (event as CustomEvent<string>).detail });
    }
    function openAccount() {
      setOpen({ kind: "account" });
    }
    function openNotice(event: Event) {
      setOpen({ kind: "notice", message: (event as CustomEvent<string>).detail });
    }
    window.addEventListener("visit:open", openVisit);
    window.addEventListener("account:open", openAccount);
    window.addEventListener("notice:show", openNotice);
    return () => {
      window.removeEventListener("visit:open", openVisit);
      window.removeEventListener("account:open", openAccount);
      window.removeEventListener("notice:show", openNotice);
    };
  }, []);

  const close = () => setOpen(null);

  function dismissImport() {
    try {
      window.sessionStorage.setItem(importDismissedKey, "1");
    } catch {
      // Without sessionStorage the question simply comes back on reload.
    }
    setImportDismissed(true);
  }

  if (open?.kind === "visit") {
    const spot = allSpots.find((s) => s.id === open.spotId);
    return spot ? <VisitDialog spot={spot} onClose={close} /> : null;
  }
  if (open?.kind === "account") {
    return <AccountDialog onClose={close} />;
  }
  if (open?.kind === "notice") {
    return (
      <Dialog title="お知らせ" onClose={close}>
        <p>{open.message}</p>
        <div className="dialogActions">
          <button type="button" className="primary" onClick={close}>
            閉じる
          </button>
        </div>
      </Dialog>
    );
  }
  if (me && localImportCount > 0 && !importDismissed) {
    return (
      <Dialog title="訪問記録の取り込み" onClose={dismissImport}>
        <LocalImportSection onDone={dismissImport} />
        <div className="dialogActions">
          <button type="button" onClick={dismissImport}>
            あとで
          </button>
        </div>
        <p className="dialogNote">あとで取り込む場合は、アカウントメニューから行えます。</p>
      </Dialog>
    );
  }
  return null;
}

export default DialogHost;
