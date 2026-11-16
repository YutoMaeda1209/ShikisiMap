import { useState } from "react";
import { ApiError, type Spot, type Visit } from "./api";
import { formatDateTime, fromLocalInputValue, toLocalInputValue } from "./datetime";
import Dialog from "./Dialog";
import { DuplicateVisitError } from "./visitStore";
import { useVisits } from "./visitsContext";

const maxMemoLength = 1000;

function errorMessage(err: unknown): string {
  if (err instanceof DuplicateVisitError) {
    return "同じ日時の訪問記録がすでにあります。";
  }
  if (err instanceof ApiError && err.status === 401) {
    return "ログインの有効期限が切れました。ページを再読み込みしてください。";
  }
  if (err instanceof ApiError && err.status === 422) {
    return "入力内容を確認してください。未来の日時は記録できません。";
  }
  return "保存できませんでした。時間をおいてもう一度お試しください。";
}

type VisitFormProps = {
  initialVisitedAt: string;
  initialMemo: string;
  submitLabel: string;
  onSubmit: (visitedAt: string, memo: string | null) => Promise<void>;
  onCancel?: () => void;
};

function VisitForm({ initialVisitedAt, initialMemo, submitLabel, onSubmit, onCancel }: VisitFormProps) {
  const [visitedAt, setVisitedAt] = useState(initialVisitedAt);
  const [memo, setMemo] = useState(initialMemo);
  const [error, setError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    const iso = fromLocalInputValue(visitedAt);
    if (!iso) {
      setError("訪問日時を入力してください。");
      return;
    }
    setIsSaving(true);
    setError(null);
    try {
      await onSubmit(iso, memo.trim() === "" ? null : memo);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <form className="dialogBody" style={{ padding: 0 }} onSubmit={submit}>
      <label>
        訪問日時
        <input
          type="datetime-local"
          value={visitedAt}
          max={toLocalInputValue(new Date(Date.now() + 24 * 60 * 60 * 1000))}
          onChange={(e) => setVisitedAt(e.target.value)}
          required
        />
      </label>
      <label>
        メモ (自分だけが見られます)
        <textarea value={memo} maxLength={maxMemoLength} onChange={(e) => setMemo(e.target.value)} />
      </label>
      {error ? (
        <p className="dialogError" role="alert">
          {error}
        </p>
      ) : null}
      <div className="dialogActions">
        <button type="submit" className="primary" disabled={isSaving}>
          {submitLabel}
        </button>
        {onCancel ? (
          <button type="button" onClick={onCancel} disabled={isSaving}>
            キャンセル
          </button>
        ) : null}
      </div>
    </form>
  );
}

type VisitRowProps = {
  visit: Visit;
};

function VisitRow({ visit }: VisitRowProps) {
  const { updateVisit, removeVisit } = useVisits();
  const [mode, setMode] = useState<"view" | "edit" | "confirmDelete">("view");
  const [error, setError] = useState<string | null>(null);

  async function remove() {
    try {
      await removeVisit(visit.id);
    } catch (err) {
      setError(errorMessage(err));
      setMode("view");
    }
  }

  if (mode === "edit") {
    return (
      <li>
        <VisitForm
          initialVisitedAt={toLocalInputValue(new Date(visit.visitedAt))}
          initialMemo={visit.memo ?? ""}
          submitLabel="保存"
          onSubmit={async (visitedAt, memo) => {
            await updateVisit(visit.id, { visitedAt, memo });
            setMode("view");
          }}
          onCancel={() => setMode("view")}
        />
      </li>
    );
  }

  return (
    <li>
      <strong>{formatDateTime(visit.visitedAt)}</strong>
      {visit.memo ? <span className="visitHistoryMemo">{visit.memo}</span> : null}
      {error ? (
        <p className="dialogError" role="alert">
          {error}
        </p>
      ) : null}
      {mode === "confirmDelete" ? (
        <div className="dialogActions">
          <span>この記録を削除しますか?</span>
          <button type="button" className="danger" onClick={remove}>
            削除する
          </button>
          <button type="button" onClick={() => setMode("view")}>
            キャンセル
          </button>
        </div>
      ) : (
        <div className="dialogActions">
          <button type="button" onClick={() => setMode("edit")}>
            編集
          </button>
          <button type="button" className="danger" onClick={() => setMode("confirmDelete")}>
            削除
          </button>
        </div>
      )}
    </li>
  );
}

type VisitDialogProps = {
  spot: Spot;
  onClose: () => void;
};

// Records a visit to the spot and lists the earlier ones.
function VisitDialog({ spot, onClose }: VisitDialogProps) {
  const { visits, addVisit, storage } = useVisits();
  const spotVisits = visits.filter((v) => v.spotId === spot.id);
  // Remounting the form after a save resets it to the current time.
  const [formKey, setFormKey] = useState(0);
  const [notice, setNotice] = useState<string | null>(null);

  return (
    <Dialog title={spot.name} onClose={onClose}>
      <section>
        <h3>訪問を記録</h3>
        {storage === "local" ? (
          <p className="dialogNote">
            ログインしていないため、記録はこの端末にだけ保存されます。ログインするとアカウントに取り込めます。
          </p>
        ) : null}
        <VisitForm
          key={formKey}
          initialVisitedAt={toLocalInputValue(new Date())}
          initialMemo=""
          submitLabel="記録する"
          onSubmit={async (visitedAt, memo) => {
            const result = await addVisit({ spotId: spot.id, visitedAt, memo });
            setNotice(result.created.length === 0 ? "同じ日時の訪問記録がすでにあります。" : "記録しました。");
            setFormKey((k) => k + 1);
          }}
        />
        {notice ? (
          <p className="dialogNote" role="status">
            {notice}
          </p>
        ) : null}
      </section>
      <hr className="dialogDivider" />
      <section>
        <h3>訪問履歴 ({spotVisits.length} 件)</h3>
        {spotVisits.length === 0 ? (
          <p className="dialogNote">まだ記録がありません。</p>
        ) : (
          <ul className="visitHistory">
            {spotVisits.map((visit) => (
              <VisitRow key={visit.id} visit={visit} />
            ))}
          </ul>
        )}
      </section>
    </Dialog>
  );
}

export default VisitDialog;
