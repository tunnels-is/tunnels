import { client } from "./client";

export const getBackendState = async () => {
    const response = await client.post("/getState", {});
    return response.data;
};

export const setConfig = async (config) => {
    const response = await client.post("/setConfig", config);
    return response.data;
};
