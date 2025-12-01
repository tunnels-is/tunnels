import { forwardToController } from "./client";

export const getConnectedDevices = async () => {
  const data = await forwardToController("POST", "/v3/devices", {}, true);
  return data;
};

export const getDevices = async ({ offset, limit }) => {
  const data = await forwardToController(
    "POST",
    "/v3/device/list",
    { Offset: offset, Limit: limit },
    true
  );
  return data;
};

export const deleteDevice = async (id) => {
  const data = await forwardToController(
    "POST",
    "/v3/device/delete",
    { DID: id },
    true
  );
  return data;
};

export const updateDevice = async (device) => {
  const data = await forwardToController(
    "POST",
    "/v3/device/update",
    { Device: device },
    true
  );
  return data;
};

export const createDevice = async (device) => {
  const data = await forwardToController(
    "POST",
    "/v3/device/create",
    { Device: device },
    true
  );
  return data;
};
