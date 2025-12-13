import { useSetAtom } from "jotai";
import { configAtom } from "../stores/configStore";
import { userAtom } from "../stores/userStore";
import { activeTunnelsAtom } from "../stores/tunnelStore";
import { getBackendState } from "../api/app";
import { useEffect } from "react";

export const useInitialState = () => {
  const setConfig = useSetAtom(configAtom);
  const setUser = useSetAtom(userAtom);
  const setActiveTunnels = useSetAtom(activeTunnelsAtom);

  useEffect(() => {
    const fetchState = async () => {
      try {
        const data = await getBackendState();
        if (data) {
          if (data.Config) {
            setConfig(data.Config);
          }
          if (data.ActiveTunnels) {
            setActiveTunnels(data.ActiveTunnels);
          }
          // Note: User might not be in GetBackendState response based on legacy code,
          // but if it is, we can set it.
          // Legacy code commented out STATE.User update, so maybe we rely on persistence or Login.

          // We can also handle other state items here if needed.
        }
      } catch (error) {
        console.error("Failed to fetch backend state:", error);
      }
    };

    fetchState();
  }, [setConfig, setUser, setActiveTunnels]);
};
