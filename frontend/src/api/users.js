import { forwardToController } from "./client";

export const getUsers = async ({ offset, limit }) => {
  const data = await forwardToController("POST", "/v3/user/list", { Offset: offset, Limit: limit }, true);
  console.log("getUsers() = ", data);
  return data;
};



export const updateUser = async (user) => {
  const response = await forwardToController("POST", "/v3/user/update", {
    Email: user.Email,
    UID: user.ID,
    APIKey: user.APIKey,
  }, true);
  return response;
};
export const deleteUser = async (user) => {
  const response = await forwardToController("POST", "/v3/user/delete", {
    TargetUserID: user.ID,
  }, true);
  return response;
};