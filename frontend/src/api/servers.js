import { forwardToController } from "./client";

export const getServers = async () => {
  const data = await forwardToController("POST", "/v3/servers", { StartIndex: 0 }, true);
  return data || [];
};

export const createServer = async ({ serverData }) => {
  return forwardToController("POST", "/v3/server/create", { Server: serverData }, true);
};

export const updateServer = async ({ serverData }) => {
  return forwardToController("POST", "/v3/server/update", { Server: serverData }, true);
};

export const deleteServer = async ({ serverId }) => {
  return forwardToController("POST", "/v3/server/delete", { ID: serverId }, true);
};
