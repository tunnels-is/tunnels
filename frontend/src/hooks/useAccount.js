import { useMutation, useQueryClient } from "@tanstack/react-query";
import { updateUser, activateLicense } from "../api/account";
import { useSetAtom } from "jotai";
import { userAtom } from "../stores/userStore";

export const useUpdateUser = () => {
    const setUser = useSetAtom(userAtom);
    return useMutation({
        mutationFn: updateUser,
        onSuccess: (data) => {
            setUser(data);
        },
    });
};

export const useActivateLicense = () => {
    const setUser = useSetAtom(userAtom);
    return useMutation({
        mutationFn: activateLicense,
        onSuccess: (data) => {
            setUser(data);
        },
    });
};
