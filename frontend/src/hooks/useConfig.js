import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { setConfig, getBackendState } from "../api/app";
import { toast } from "sonner";
import { useAtom, useSetAtom } from "jotai";
import { configAtom } from "../stores/configStore";
import { loadingAtom, toggleLoadingAtom } from "../stores/uiStore";

export const useConfig = () => {
    return useQuery({
        queryKey: ["config"],
        queryFn: async () => {
            const state = await getBackendState();
            return state?.Config;
        }
    });
};

export const useSaveConfig = () => {
    const queryClient = useQueryClient();
    const setConfigAtom = useSetAtom(configAtom);
    const setLoading = useSetAtom(toggleLoadingAtom);

    return useMutation({
        mutationFn: async (newConfig) => {
            setLoading({ show: true, msg: "Saving config..." });
            try {
                const result = await setConfig(newConfig);
                return result;
            } finally {
                setLoading(undefined);
            }
        },
        onSuccess: (data, variables) => {
            queryClient.invalidateQueries({ queryKey: ["config"] });
            setConfigAtom(variables);
            toast.success("Config saved");
        },
        onError: (error) => {
            toast.error("Failed to save config: " + error.message);
        }
    });
};
