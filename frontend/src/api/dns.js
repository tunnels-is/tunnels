import { client } from "./client";

export const getDNSStats = async () => {
    const response = await client.post("/getDNSStats", {});
    return response.data;
};
