import { forwardToController } from "./client";



export const activateLicense = async (key) => await forwardToController("POST", "/v3/user/license", { Key: key });
