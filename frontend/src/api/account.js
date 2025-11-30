import { client } from "./client";

export const updateUser = async (user) => {
    const response = await client.post("/v3/user/update", {
        Email: user.Email,
        APIKey: user.APIKey,
        // Add other fields if necessary
    });
    return response.data;
};

export const activateLicense = async (key) => {
    const response = await client.post("/v3/user/license", { Key: key });
    return response.data;
};
