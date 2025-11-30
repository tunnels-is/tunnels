import { getBackendState } from "@/api/app";
import { atom } from "jotai";
import { atomWithStorage } from "jotai/utils";

export const configAtom = atomWithStorage("config", {
    DarkMode: false,
    OpenUI: false,
    ControlServers: [{
        ID: "tunnels",
        Host: "api.tunnels.is",
        Port: "443",
        HTTPS: true,
        ValidateCertificate: true,
        CertificatePath: "",
    }]
});

configAtom.onMount = (set) => {
    getBackendState().then((state) => {
        if (state?.Config) {
            set(state.Config);
        }
    }).catch((err) => {
        console.error("Failed to fetch backend state:", err);
    });
};


export const controlServersAtom = atom((get) => get(configAtom).ControlServers || []);

export const controlServerAtom = atomWithStorage("control-server", {
    ID: "tunnels",
    Host: "api.tunnels.is",
    Port: "443",
    HTTPS: true,
    ValidateCertificate: true,
    CertificatePath: "",
});