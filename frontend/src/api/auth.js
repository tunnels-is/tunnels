import { forwardToController, client } from "./client";

export const logout = async (data) => await forwardToController("POST", "/v3/user/logout", data);

export const getUser = async () => await forwardToController("GET", "/v3/user/me");

export const getAccounts = async () => {
  const response = await client.post("/getUsers");
  return response.data;
};

export const delAccount = async (hash) => {
  const response = await client.post("/delUser", { Hash: hash });
  return response.data;
};

export const setUser = async (user) => {
  const response = await client.post("/setUser", user);
  return response.data;
};

export const loginUser = async (data) => await forwardToController("POST", "/v3/user/login", data);
export const registerUser = async (data) => await forwardToController("POST", "/v3/user/create", data);
export const enableUser = async (data) => await forwardToController("POST", "/v3/user/enable", data);
export const resetPassword = async (data) => await forwardToController("POST", "/v3/user/reset/password", data);
export const sendResetCode = async (data) => await forwardToController("POST", "/v3/user/reset/code", data);

