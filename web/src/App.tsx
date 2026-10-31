import { useEffect, useState } from "react";
import { Home } from "./components/Home";
import { GamePage } from "./components/GamePage";

// Two routes don't justify a router dependency: "/" and "/g/:code".
function currentCode(): string | null {
  const m = location.pathname.match(/^\/g\/([A-Za-z0-9]{4})\/?$/);
  return m ? m[1].toUpperCase() : null;
}

export function navigate(path: string) {
  history.pushState(null, "", path);
  dispatchEvent(new PopStateEvent("popstate"));
}

export function App() {
  const [code, setCode] = useState(currentCode);
  useEffect(() => {
    const onPop = () => setCode(currentCode());
    addEventListener("popstate", onPop);
    return () => removeEventListener("popstate", onPop);
  }, []);

  return (
    <div className="app">
      <header className="topbar">
        <a
          href="/"
          className="logo"
          onClick={(e) => {
            e.preventDefault();
            navigate("/");
          }}
        >
          Jotto
        </a>
      </header>
      <main>{code ? <GamePage key={code} code={code} /> : <Home />}</main>
    </div>
  );
}
