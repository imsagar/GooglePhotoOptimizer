import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { Layout } from "./components/Layout";
import { Login } from "./pages/Login";
import { Dashboard } from "./pages/Dashboard";
import { Videos } from "./pages/Videos";
import { Jobs } from "./pages/Jobs";
import { LocalFiles } from "./pages/LocalFiles";
import { Settings } from "./pages/Settings";

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route element={<Layout />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/videos" element={<Videos />} />
          <Route path="/jobs" element={<Jobs />} />
          <Route path="/local-files" element={<LocalFiles />} />
          <Route path="/settings" element={<Settings />} />
        </Route>
        <Route path="*" element={<Navigate to="/" />} />
      </Routes>
    </BrowserRouter>
  );
}
