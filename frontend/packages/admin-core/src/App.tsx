import { Navigate, Route, Routes } from "react-router-dom";
import Login from "./pages/Login";
import NoAccess from "./pages/NoAccess";
import MainLayout from "./layout/MainLayout";
import { AccessProvider, useAccess } from "./runtime/access";
import { useModuleRegistry, useSessionSnapshot } from "./runtime/context";
import { compileMenuPages } from "./module/routes";
import { renderRegisteredPage, renderRouteConflicts } from "./module/pages";

function RequireAuth({ children }: { children: JSX.Element }) {
  const session = useSessionSnapshot();
  if (!session.token) return <Navigate to="/login" replace />;
  return <AccessProvider>{children}</AccessProvider>;
}

function AuthedApp() {
  const { menus } = useAccess();
  const registry = useModuleRegistry();
  const session = useSessionSnapshot();
  const compiled = compileMenuPages(menus);

  if (compiled.conflicts.length > 0 && compiled.pages.length === 0) {
    return (
      <Routes key={session.epoch}>
        <Route element={<MainLayout />}>
          <Route path="*" element={renderRouteConflicts(compiled.conflicts)} />
        </Route>
      </Routes>
    );
  }

  if (compiled.pages.length === 0) {
    return (
      <Routes key={session.epoch}>
        <Route element={<MainLayout />}>
          <Route path="*" element={<NoAccess />} />
        </Route>
      </Routes>
    );
  }

  const first = compiled.pages[0].path;
  const conflictByPath = new Map(compiled.conflicts.map((item) => [item.path, item]));

  return (
    <Routes key={session.epoch}>
      <Route element={<MainLayout />}>
        <Route index element={<Navigate to={first} replace />} />
        {compiled.pages.map((route) => (
          <Route
            key={route.path}
            path={route.path.replace(/^\//, "")}
            element={renderRegisteredPage(route.component, registry)}
          />
        ))}
        {compiled.conflicts.map((conflict) => (
          <Route
            key={`conflict:${conflict.path}`}
            path={conflict.path.replace(/^\//, "")}
            element={renderRouteConflicts([conflict])}
          />
        ))}
        <Route
          path="*"
          element={
            conflictByPath.size > 0 ? (
              renderRouteConflicts(compiled.conflicts)
            ) : (
              <Navigate to={first} replace />
            )
          }
        />
      </Route>
    </Routes>
  );
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/*"
        element={
          <RequireAuth>
            <AuthedApp />
          </RequireAuth>
        }
      />
    </Routes>
  );
}
