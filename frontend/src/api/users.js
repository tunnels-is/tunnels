import { forwardToController } from "./client";

export const getUsers = async ({ offset, limit }) => {
    const data = await forwardToController("POST", "/v3/user/list", { Offset: offset, Limit: limit }, true);
    return data;
};

export const adminUpdateUser = async (user) => {
    const response = await forwardToController("POST", "/v3/user/adminupdate", {
        TargetUserID: user.ID,
        Email: user.Email,
        Disabled: user.Disabled,
        IsManager: user.IsManager,
        Trial: user.Trial,
        SubExpiration: user.SubExpiration
    }, true);
    return response;
};
