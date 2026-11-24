import { useEffect, useRef } from 'react';
import { FormField } from '../components/FormField';
import { Problem } from '../components/Problem';
import { Progress } from '../components/Progress';
import { pickAll, setAlbumField, togglePick, useAdd, type AlbumKey } from '../state/add';

const fields: readonly { key: AlbumKey; label: string; placeholder?: string }[] = [
  { key: 'album', label: 'Album' },
  { key: 'artist', label: 'Artist' },
  { key: 'released', label: 'Released', placeholder: "each track's own" },
  { key: 'genre', label: 'Genre', placeholder: "each track's own" },
];

function length(seconds: number): string {
  const whole = Math.max(0, Math.round(seconds));
  return `${String(Math.floor(whole / 60))}:${String(whole % 60).padStart(2, '0')}`;
}

export function albumControls(root: HTMLElement | null): HTMLElement[] {
  return [...(root?.querySelectorAll<HTMLElement>('.albumreview input, .albumreview button') ?? [])];
}

export function AlbumReview() {
  const { album, albumForm, albumErrors, picked, cover, error } = useAdd();
  const first = useRef<HTMLInputElement>(null);

  useEffect(() => {
    first.current?.focus();
  }, []);

  if (album === null) {
    return null;
  }

  return (
    <div className="albumreview">
      <div className="dim">{album.note}</div>
      {cover !== '' && (
        <div className="cover-box">
          <img src={`/api/add/${encodeURIComponent(album.id)}/cover?v=${cover.slice(0, 16)}`} alt="Cover" />
        </div>
      )}
      <div className="form">
        {fields.map((f, at) => (
          <FormField key={f.key} name={f.key} label={f.label} error={albumErrors[f.key]}>
            {(control) => (
              <input
                {...control}
                ref={at === 0 ? first : undefined}
                className="edit"
                type="text"
                dir="auto"
                autoComplete="off"
                spellCheck={false}
                placeholder={f.placeholder}
                value={albumForm[f.key]}
                onChange={(e) => {
                  setAlbumField(f.key, e.target.value);
                }}
              />
            )}
          </FormField>
        ))}
      </div>
      <div className="pickrow">
        <span>{`${String(picked.length)} of ${String(album.tracks.length)} selected`}</span>
        <button
          type="button"
          className="btn"
          onClick={() => {
            pickAll(true);
          }}
        >
          Select all
        </button>
        <button
          type="button"
          className="btn"
          onClick={() => {
            pickAll(false);
          }}
        >
          Select none
        </button>
      </div>
      <div className="tracklist">
        {album.tracks.map((t) => (
          <label key={t.number} className="trackrow">
            <input
              type="checkbox"
              aria-label={`${String(t.number)}. ${t.title}`}
              checked={picked.includes(t.number)}
              onChange={() => {
                togglePick(t.number);
              }}
            />
            <span className="dim">{`${String(t.number)}. `}</span>
            <bdi dir="auto">{t.title}</bdi>
            <span className="dim tracklength">{length(t.seconds)}</span>
          </label>
        ))}
      </div>
      {error !== '' && <Problem text={error} />}
    </div>
  );
}

export function AlbumProgressView() {
  const { progress } = useAdd();
  return (
    <>
      <div className="busy">
        {progress === null ? 'Downloading…' : <>Downloading <bdi dir="auto">{progress.title}</bdi>…</>}
      </div>
      <Progress done={progress?.done ?? 0} total={progress?.total ?? 0} label="Adding the album" />
      <div className="ask-body">esc cancels.</div>
    </>
  );
}
