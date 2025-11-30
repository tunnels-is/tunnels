import { atom } from "jotai";
import { atomWithStorage } from "jotai/utils";

export const loadingAtom = atom(undefined);
export const debugAtom = atomWithStorage("debug", false);

export const toggleLoadingAtom = atom(
    null,
    (get, set, loadingState) => {
        set(loadingAtom, loadingState);
    }
);
