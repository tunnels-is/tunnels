import { forwardToController } from "./client";

export const updateUser = async (user) => await forwardToController("POST", "/v3/user/update", {
    Email: user.Email,
    APIKey: user.APIKey,
    // Add other fields if necessary
});

export const activateLicense = async (key) => await forwardToController("POST", "/v3/user/license", { Key: key });
