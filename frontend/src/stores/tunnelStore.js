import { atom } from "jotai";
import { atomWithStorage } from "jotai/utils";

export const activeTunnelsAtom = atom([]);
export const tunnelsAtom = atomWithStorage("tunnels", []);
