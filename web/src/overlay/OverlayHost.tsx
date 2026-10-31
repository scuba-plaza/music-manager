import { Ask } from '../dialogs/Ask';
import { BulkForm } from '../dialogs/BulkForm';
import { CopyList } from '../dialogs/CopyList';
import { Help } from '../dialogs/Help';
import { PlaylistPicker } from '../dialogs/PlaylistPicker';
import { SortMenu } from '../dialogs/SortMenu';
import { useApp } from '../state/store';

export function OverlayHost() {
  const overlay = useApp((s) => s.overlay);
  const view = useApp((s) => s.lib?.view);
  if (overlay === null) {
    return null;
  }
  switch (overlay.kind) {
    case 'ask':
      return <Ask head={overlay.head} body={overlay.body} yes={overlay.yes} onYes={overlay.onYes} />;
    case 'help':
      return <Help />;
    case 'sort':
      return view === undefined ? null : <SortMenu sort={view.sort} />;
    case 'playlist':
      return <PlaylistPicker ids={overlay.ids} lists={view?.playlists ?? []} />;
    case 'copy':
      return <CopyList id={overlay.id} title={overlay.title} choices={overlay.choices} />;
    case 'bulk':
      return <BulkForm ids={overlay.ids} head={overlay.head} />;
    default:
      return null;
  }
}
