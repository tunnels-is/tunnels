import { useAtomValue } from "jotai";
import { configAtom } from "../stores/configStore";
import { useSaveConfig } from "./useConfig";
import { useCallback } from "react";

export const useSaveControlServer = () => {
    const config = useAtomValue(configAtom);
    const saveConfigMutation = useSaveConfig();

    return useCallback((newAuth) => {
        const newConfig = { ...config };
        if (!newConfig.ControlServers) newConfig.ControlServers = [];
        const newControlServers = [...newConfig.ControlServers];
        let index = newControlServers.findIndex(s => s.ID === newAuth.ID);
        if (index !== -1) {
            newControlServers[index] = { ...newAuth };
        } else {
            newControlServers.push(newAuth);
        }
        newConfig.ControlServers = newControlServers;
        saveConfigMutation.mutate(newConfig);
    }, [config, saveConfigMutation]);
};
