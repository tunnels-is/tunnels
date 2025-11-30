import axios from "axios";
import { toast } from "sonner";
import { getDefaultStore } from "jotai";
import { controlServerAtom } from "@/stores/configStore";
import { userAtom } from "@/stores/userStore";


const baseURL = import.meta.env.VITE_BASE_URL;

export const client = axios.create({
  baseURL: baseURL + "/v1/method",
  headers: {
    "Content-Type": "application/json",
  },
  timeout: 30000,
});

// client.interceptors.request.use((config) => {
//   // You can add auth tokens here if needed, or handle them in the specific API methods
//   // For now, we'll keep it simple as the original code passed tokens in the body
//   return config;
// });

// client.interceptors.response.use(
//   (response) => {
//     return response;
//   },
//   (error) => {
//     if (error.response?.status === 401) {
//       // Handle unauthorized access
//       // This might need to interact with the global store or router to redirect
//       console.warn("Unauthorized access");
//     }
//     return Promise.reject(error);
//   }
// );

export const handleApiError = (error) => {
  console.error(error);
  if (error?.message === "Network Error") {
    toast.success("Tunnel connected, network changed");
    return;
  }

  const message = error?.response?.data?.Error || error?.response?.data?.Message || "Unknown error";
  toast.error(message);
};

/**
 * call the control server
 * @param {"POST"|"GET"} method HTTP(S) method
 * @param {string} path route on the control server
 * @param {any} data body
 * @param {boolean} auth authenticate the request or not
 * @returns {Promise<Record<string, any> | boolean>}
 */
export const forwardToController = async (method, path, data, auth = false) => {
  const store = getDefaultStore();
  const controlServer = store.get(controlServerAtom);
  const user = store.get(userAtom);
  console.log("control server: ", controlServer);
  const body = {
    Server: controlServer,
    Path: path,
    Method: method,
    JSONData: auth ? { ...data, UID: user.ID, DeviceToken: user.DeviceToken.DT, Email: user.Email } : data,
    Timeout: 20000,
  };
  console.log(body)
  const response = await client.post("/forwardToController", body);
  return response.data;
};

