import { forwardToController } from "./client";

export const login = async (credentials) => {
  // Adjust endpoint based on actual login flow, assuming standard for now
  const response = await forwardToController("POST", "/v3/user/login", credentials);
  console.log(response);
  return response.data;
};

export const logout = async (data) => {
  const response = await forwardToController("POST", "/v3/user/logout", data);
  console.log(response);
  return response.data;
};

export const getUser = async () => {
  const response = await forwardToController("GET", "/v3/user/me");
  return response.data;
};

export const getUsers = async () => {
  const response = await client.post("/getUsers");
  return response.data;
};

export const deleteUser = async (hash) => {
  const response = await client.post("/delUser", { Hash: hash });
  return response.data;
};



export const loginUser = async (data) => {
  return await forwardToController("POST", "/v3/user/login", data);
};

export const registerUser = async (data) => {
  return await forwardToController("POST", "/v3/user/create", data);
};

export const enableUser = async (data) => {
  return await forwardToController("POST", "/v3/user/enable", data);
};

export const resetPassword = async (data) => {
  return await forwardToController("POST", "/v3/user/reset/password", data);
};

export const sendResetCode = async (data) => {
  return await forwardToController("POST", "/v3/user/reset/code", data);
};

export const saveUserToDisk = async (user) => {
  const response = await client.post("/setUser", user);
  return response.data;
};

