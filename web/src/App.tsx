import { useEffect } from 'react';
import { Footer } from './components/Footer';
import { Header } from './components/Header';
import { Split } from './components/Split';
import { installKeys } from './keys/dispatch';
import { connect, loadInfo, loadLibrary, run } from './state/actions';
import { useApp } from './state/store';
import './styles/tokens.css';
import './styles/app.css';

function ignoreJob(): void {
  return;
}

export function App() {
  const title = useApp((s) => s.lib?.view.sort.title ?? 'Library');

  useEffect(() => {
    void loadInfo();
    void loadLibrary();
    return connect(ignoreJob);
  }, []);

  useEffect(() => installKeys(document, run), []);

  return (
    <div className="app">
      <Header />
      <Split
        leftTitle={title}
        rightTitle="Details"
        left={<div className="pane-body" data-testid="tree-pane" />}
        right={<div className="pane-body" data-testid="inspector-pane" />}
      />
      <Footer />
    </div>
  );
}
