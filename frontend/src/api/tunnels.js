import { client } from "./client";

export const getTunnels = async () => {
    const response = await client.post("/getState", {});
    return response.data?.Tunnels || [];
};

export const createTunnel = async () => {
    const response = await client.post("/createTunnel");
    return response.data;
};

export const deleteTunnel = async (tunnel) => {
    const response = await client.post("/deleteTunnel", tunnel);
    return response.data;
};

export const updateTunnel = async ({ tunnel, oldTag }) => {
    const response = await client.post("/setTunnel", { Meta: tunnel, OldTag: oldTag });
    return response.data;
};

export const connectTunnel = async (connectionRequest) => {
    const response = await client.post("/connect", connectionRequest);
    return response.data;
};

export const disconnectTunnel = async (id) => {
    const response = await client.post("/disconnect", { ID: id });
    return response.data;
};

