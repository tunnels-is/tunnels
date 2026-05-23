import { HashRouter, Route, Routes, Navigate } from 'react-router-dom';
import { isLoggedIn } from './auth';
import Login from './pages/Login';
import Layout from './pages/Layout';
import Users from './pages/Users';
import UserDetail from './pages/UserDetail';
import Devices from './pages/Devices';
import DeviceDetail from './pages/DeviceDetail';
import Groups from './pages/Groups';
import GroupDetail from './pages/GroupDetail';
import Servers from './pages/Servers';
import ServerDetail from './pages/ServerDetail';

function RequireAuth({ children }) {
  if (!isLoggedIn()) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

export default function App() {
  return (
    <HashRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route
          path="/*"
          element={
            <RequireAuth>
              <Layout />
            </RequireAuth>
          }
        >
          <Route index element={<Navigate to="/users" replace />} />
          <Route path="users" element={<Users />} />
          <Route path="users/:id" element={<UserDetail />} />
          <Route path="devices" element={<Devices />} />
          <Route path="devices/:id" element={<DeviceDetail />} />
          <Route path="groups" element={<Groups />} />
          <Route path="groups/:id" element={<GroupDetail />} />
          <Route path="servers" element={<Servers />} />
          <Route path="servers/:id" element={<ServerDetail />} />
          <Route path="*" element={<Navigate to="/users" replace />} />
        </Route>
      </Routes>
    </HashRouter>
  );
}
