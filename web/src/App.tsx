import { useEffect } from 'react';
import { Footer } from './components/Footer';
import { Header } from './components/Header';
import { Inspector } from './components/Inspector';
import { Split } from './components/Split';
import { Tree } from './components/Tree';
import { installKeys } from './keys/dispatch';
import { OverlayHost } from './overlay/OverlayHost';
import { connect, loadInfo, loadLibrary, run } from './state/actions';
import { isTrack } from './state/inspector';
import { loadProfiles } from './state/profiles';
import { useApp } from './state/store';
import './styles/tokens.css';
import './styles/app.css';

function ignoreJob(): void {
  return;
}

export function App() {
  const title = useApp((s) => s.lib?.view.sort.title ?? 'Library');
  const detailTitle = useApp((s) => (s.detail === null || isTrack(s.detail) ? 'Details' : s.detail.title));

  useEffect(() => {
    void loadInfo();
    void loadProfiles();
    void loadLibrary();
    return connect(ignoreJob);
  }, []);

  useEffect(() => installKeys(document, run), []);

  return (
    <div className="app">
      <Header />
      <Split
        leftTitle={title}
        rightTitle={detailTitle}
        left={<Tree />}
        right={<Inspector />}
      />
      <Footer />
      <OverlayHost />
    </div>
  );
}
