import { atom } from "jotai";
import { atomWithStorage } from "jotai/utils";

// Persist user state in localStorage (or sessionStorage if preferred)
export const userAtom = atomWithStorage("user", null);

export const isAuthenticatedAtom = atom((get) => get(userAtom) !== null);

export const defaultEmailAtom = atomWithStorage("default-email", "");
export const defaultDeviceNameAtom = atomWithStorage("default-device-name", "");
