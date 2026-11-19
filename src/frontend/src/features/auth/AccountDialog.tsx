import { useState } from "react";
import { useAuth } from "./authContext";
import Dialog from "../../components/Dialog";
import { useMapData } from "../spots/mapDataContext";
import { profileUrl } from "../../lib/route";
import {
  downloadBlob,
  exportAccountVisits,
  exportLocalVisits,
  type ExportFormat,
} from "../visits/visitStore";
import { useVisits } from "../visits/visitsContext";

const maxDisplayNameLength = 50;

type AccountDialogProps = {
  onClose: () => void;
};

function ExportSection() {
  const { visits, storage } = useVisits();
  const { allSpots } = useMapData();
  const [error, setError] = useState<string | null>(null);

  async function exportVisits(format: ExportFormat) {
    setError(null);
    try {
      const blob =
        storage === "account" ? await exportAccountVisits(format) : exportLocalVisits(format, allSpots);
      downloadBlob(blob, `shikisimap-visits.${format}`);
    } catch {
      setError("エクスポートできませんでした。");
    }
  }

  return (
    <section>
      <h3>訪問記録のエクスポート</h3>
      <p className="dialogNote">
        {storage === "account" ? "アカウント" : "この端末"}の訪問記録 {visits.length} 件をファイルに保存します。
      </p>
      <div className="dialogActions">
        <button type="button" onClick={() => exportVisits("csv")} disabled={visits.length === 0}>
          CSV
        </button>
        <button type="button" onClick={() => exportVisits("json")} disabled={visits.length === 0}>
          JSON
        </button>
      </div>
      {error ? (
        <p className="dialogError" role="alert">
          {error}
        </p>
      ) : null}
    </section>
  );
}

/** Lets a signed-in user import or discard the records kept in this browser. */
export function LocalImportSection({ onDone }: { onDone?: () => void }) {
  const { localImportCount, importLocal, discardLocal } = useVisits();
  const [state, setState] = useState<"idle" | "busy" | "confirmDiscard">("idle");
  const [message, setMessage] = useState<string | null>(null);

  async function runImport() {
    setState("busy");
    try {
      const result = await importLocal();
      setMessage(
        `${result.created.length} 件を取り込みました。` +
          (result.skipped > 0 ? `${result.skipped} 件は登録済みのため取り込みませんでした。` : "")
      );
      onDone?.();
    } catch {
      // A record of a spot that no longer exists makes the server refuse all of them.
      setMessage("取り込めませんでした。記録はこの端末に残っています。");
    } finally {
      setState("idle");
    }
  }

  if (localImportCount === 0) {
    return message ? (
      <section>
        <p role="status">{message}</p>
      </section>
    ) : null;
  }

  return (
    <section>
      <p>
        この端末にログイン前の訪問記録が {localImportCount} 件あります。アカウントに取り込みますか?
      </p>
      {state === "confirmDiscard" ? (
        <div className="dialogActions">
          <span>この端末の記録を破棄しますか? 元に戻せません。</span>
          <button
            type="button"
            className="danger"
            onClick={() => {
              discardLocal();
              setState("idle");
              onDone?.();
            }}
          >
            破棄する
          </button>
          <button type="button" onClick={() => setState("idle")}>
            キャンセル
          </button>
        </div>
      ) : (
        <div className="dialogActions">
          <button type="button" className="primary" onClick={runImport} disabled={state === "busy"}>
            取り込む
          </button>
          <button type="button" className="danger" onClick={() => setState("confirmDiscard")} disabled={state === "busy"}>
            破棄する
          </button>
        </div>
      )}
      {message ? (
        <p className="dialogError" role="alert">
          {message}
        </p>
      ) : null}
    </section>
  );
}

function ProfileSection() {
  const { me, updateMe } = useAuth();
  const [displayName, setDisplayName] = useState(me?.displayName ?? "");
  const [message, setMessage] = useState<{ text: string; isError: boolean } | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  if (!me) return null;

  async function save(patch: Parameters<typeof updateMe>[0], done: string) {
    setIsSaving(true);
    setMessage(null);
    try {
      await updateMe(patch);
      setMessage({ text: done, isError: false });
    } catch {
      setMessage({ text: "保存できませんでした。", isError: true });
    } finally {
      setIsSaving(false);
    }
  }

  const trimmed = displayName.trim();
  const url = profileUrl(me.id);

  return (
    <section>
      <p className="dialogNote">{me.email}</p>
      <form
        className="dialogBody"
        style={{ padding: 0 }}
        onSubmit={(event) => {
          event.preventDefault();
          void save({ displayName: trimmed }, "表示名を保存しました。");
        }}
      >
        <label>
          表示名 (公開プロフィールに表示されます)
          <input
            type="text"
            value={displayName}
            maxLength={maxDisplayNameLength}
            onChange={(e) => setDisplayName(e.target.value)}
            required
          />
        </label>
        <div className="dialogActions">
          <button type="submit" disabled={isSaving || trimmed === "" || trimmed === me.displayName}>
            表示名を保存
          </button>
        </div>
      </form>
      <label className="inline">
        <input
          type="checkbox"
          checked={me.profilePublic}
          disabled={isSaving}
          onChange={(e) =>
            void save(
              { profilePublic: e.target.checked },
              e.target.checked ? "プロフィールを公開しました。" : "プロフィールを非公開にしました。"
            )
          }
        />
        プロフィールを公開する
      </label>
      {me.profilePublic ? (
        <p className="profileUrl">
          表示名と訪問済みスポットの一覧が公開されます (訪問日時とメモは公開されません)。
          <br />
          <a href={url} target="_blank" rel="noopener noreferrer">
            {url}
          </a>
        </p>
      ) : (
        <p className="dialogNote">公開すると、表示名と訪問済みスポットの一覧を URL で共有できます。</p>
      )}
      {message ? (
        <p className={message.isError ? "dialogError" : "dialogNote"} role={message.isError ? "alert" : "status"}>
          {message.text}
        </p>
      ) : null}
    </section>
  );
}

function SessionSection({ onClose }: { onClose: () => void }) {
  const { logout, deleteAccount } = useAuth();
  const [isConfirmingDelete, setIsConfirmingDelete] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function run(action: () => Promise<void>, failure: string) {
    setError(null);
    try {
      await action();
      onClose();
    } catch {
      setError(failure);
    }
  }

  return (
    <section>
      {isConfirmingDelete ? (
        <>
          <p>
            アカウントと、アカウントに保存した訪問記録をすべて削除します。元に戻せません。
          </p>
          <div className="dialogActions">
            <button
              type="button"
              className="danger"
              onClick={() => run(deleteAccount, "アカウントを削除できませんでした。")}
            >
              アカウントを削除する
            </button>
            <button type="button" onClick={() => setIsConfirmingDelete(false)}>
              キャンセル
            </button>
          </div>
        </>
      ) : (
        <div className="dialogActions">
          <button type="button" onClick={() => run(logout, "ログアウトできませんでした。")}>
            ログアウト
          </button>
          <button type="button" className="danger" onClick={() => setIsConfirmingDelete(true)}>
            アカウントを削除
          </button>
        </div>
      )}
      {error ? (
        <p className="dialogError" role="alert">
          {error}
        </p>
      ) : null}
    </section>
  );
}

// The user menu: login when signed out, and the account settings when signed in.
function AccountDialog({ onClose }: AccountDialogProps) {
  const { me, login } = useAuth();

  if (!me) {
    return (
      <Dialog title="ログイン" onClose={onClose}>
        <section>
          <p>
            ログインすると、訪問記録をアカウントに保存して複数の端末で見られます。ログインしなくても、記録はこの端末に保存できます。
          </p>
          <div className="dialogActions">
            <button type="button" className="primary" onClick={login}>
              Google でログイン
            </button>
          </div>
        </section>
        <hr className="dialogDivider" />
        <ExportSection />
      </Dialog>
    );
  }

  return (
    <Dialog title="アカウント" onClose={onClose}>
      <ProfileSection />
      <hr className="dialogDivider" />
      <LocalImportSection />
      <ExportSection />
      <hr className="dialogDivider" />
      <SessionSection onClose={onClose} />
    </Dialog>
  );
}

export default AccountDialog;
