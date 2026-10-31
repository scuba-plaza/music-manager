import { useEffect, useId, useRef } from 'react';
import type { DeviceView, PeekView, TrackView } from '../api/types.gen';
import { fail } from '../state/actions';
import { goToRow, stopPlaying } from '../state/edit';
import { chooseTab, isTrack, tabIndex } from '../state/inspector';
import { chooseProfile } from '../state/profiles';
import { useApp } from '../state/store';
import { Cover } from './Cover';
import { Fields } from './Fields';

function Name({ text }: { text: string }) {
  return <bdi dir="auto">{text}</bdi>;
}

function Device({ view }: { view: DeviceView }) {
  const id = useId();
  const profiles = useApp((s) => s.profiles);
  const profile = useApp((s) => s.profile);
  return (
    <div className="fields device">
      <label className="label" htmlFor={id}>
        Profile
      </label>
      <div className="value">
        <select
          id={id}
          className="profile"
          value={profile}
          onChange={(e) => {
            void chooseProfile(e.target.value);
          }}
        >
          {profiles.map((p) => (
            <option key={p.name} value={p.name}>
              {p.label}
            </option>
          ))}
        </select>
      </div>
      {view.notice !== undefined && view.notice !== '' && <div className="span warn">{view.notice}</div>}
      {view.rows.map((row) => (
        <DeviceRowView key={row.label} label={row.label} value={row.value} tail={row.tail} hint={row.hint} />
      ))}
      {(view.warnings ?? []).map((warning) => (
        <div key={warning} className="span warn">
          ! <Name text={warning} />
        </div>
      ))}
      <div className="span dim">{view.footnote}</div>
    </div>
  );
}

function DeviceRowView({ label, value, tail, hint }: { label: string; value: string; tail?: string | undefined; hint?: string | undefined }) {
  return (
    <>
      <div className="label">{label}</div>
      <div className="value">
        <Name text={value} />
        {tail !== undefined && tail !== '' && (
          <span className="dim">
            {' '}
            <Name text={tail} />
          </span>
        )}
      </div>
      {hint !== undefined && hint !== '' && <div className="span hintline">{hint}</div>}
    </>
  );
}

function TrackPane({ view }: { view: TrackView }) {
  const index = useApp((s) => tabIndex(view, s.tab));
  const tab = view.tabs[index];
  return (
    <>
      <div className="tabs" role="tablist" aria-label="Tabs">
        {view.tabs.map((t, i) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            data-tab={t.id}
            className={i === index ? 'tab on' : 'tab'}
            aria-selected={i === index}
            onClick={() => {
              chooseTab(i);
            }}
          >
            {`${String(i + 1)} ${t.name}`}
          </button>
        ))}
      </div>
      <h3 className="title">
        <Name text={view.title} />
      </h3>
      <div className="rule" />
      {tab?.notes?.map((note) => (
        <p key={note} className={tab.warn === true ? 'note warn' : 'note'}>
          {note}
        </p>
      ))}
      {tab !== undefined && tab.fields.length > 0 && <Fields fields={tab.fields} bar={tab.bar} numbers />}
      {tab?.device !== undefined && <Device view={tab.device} />}
      {index === 0 && view.coverSha !== undefined && view.coverSha !== '' && <Cover track={view.id} sha={view.coverSha} />}
      <div className="rule" />
      <div className="pathline">
        <Name text={view.path} />
      </div>
      <div className="foot">{view.dates}</div>
      <div className="foot">
        <Name text={view.about} />
      </div>
    </>
  );
}

function PeekPane({ view }: { view: PeekView }) {
  const fields = view.fields ?? [];
  return (
    <>
      <h3 className="title">
        <Name text={view.label} />
      </h3>
      <div className="rule" />
      {view.info.map((line) => (
        <p key={line} className="note">
          <Name text={line} />
        </p>
      ))}
      {view.path !== undefined && view.path !== '' && (
        <div className="pathline">
          <Name text={view.path} />
        </div>
      )}
      {fields.length > 0 && (
        <>
          <Fields fields={fields} numbers={false} />
          {view.note !== undefined && view.note !== '' && <p className="note">{view.note}</p>}
          {view.coverTrack !== undefined && view.coverTrack !== '' && <Cover track={view.coverTrack} sha={view.coverSha ?? ''} />}
          <div className="rule" />
        </>
      )}
      {view.list.length > 0 && (
        <div className="listrows">
          {view.list.map((row, i) => {
            const cols = row.cols ?? [];
            return (
              <div
                key={row.key ?? String(i)}
                className="lr"
                role="button"
                tabIndex={-1}
                onClick={() => {
                  if (row.key !== undefined) {
                    goToRow(row.key);
                  }
                }}
              >
                <div className={row.dim === true ? 'l dim' : 'l'}>
                  <Name text={row.left} />
                </div>
                <div className="c">{cols.length > 1 && <Name text={cols[0] ?? ''} />}</div>
                <div className="c mono">{cols[cols.length - 1] ?? ''}</div>
              </div>
            );
          })}
        </div>
      )}
    </>
  );
}

function Preview({ src }: { src: string }) {
  const audio = useRef<HTMLAudioElement>(null);
  useEffect(() => {
    audio.current?.play().catch((err: unknown) => {
      stopPlaying();
      fail(err);
    });
  }, [src]);
  return (
    <audio
      ref={audio}
      src={src}
      onEnded={() => {
        stopPlaying();
      }}
    />
  );
}

export function Inspector() {
  const detail = useApp((s) => s.detail);
  const detailError = useApp((s) => s.detailError);
  const hasTracks = useApp((s) => (s.lib?.view.tracks ?? 0) > 0);
  const src = useApp((s) => s.playing?.src);

  let body = null;
  if (detail === null) {
    if (detailError !== '') {
      body = <div className="empty bad">{detailError}</div>;
    } else if (hasTracks) {
      body = <div className="empty">Nothing selected</div>;
    }
  } else if (isTrack(detail)) {
    body = <TrackPane view={detail} />;
  } else {
    body = <PeekPane view={detail} />;
  }

  return (
    <div className="pane-body inspector" data-testid="inspector-pane">
      {body}
      {src !== undefined && <Preview src={src} />}
    </div>
  );
}
