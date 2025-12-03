import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  getTunnels,
  createTunnel,
  updateTunnel,
  deleteTunnel,
  connectTunnel,
  disconnectTunnel,
} from "../api/tunnels";
import { toast } from "sonner";
import { useAtomValue } from "jotai";
import { loadingAtom, toggleLoadingAtom } from "../stores/uiStore";

export const useTunnels = () => {
  return useQuery({
    queryKey: ["tunnels"],
    queryFn: getTunnels,
  });
};

export const useCreateTunnel = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createTunnel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tunnels"] });
      toast.success("Tunnel created");
    },
    onError: (error) => {
      toast.error("Failed to create tunnel: " + error.message);
    },
  });
};

export const useUpdateTunnel = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: updateTunnel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tunnels"] });
      toast.success("Tunnel saved");
    },
    onError: (error) => {
      toast.error("Failed to save tunnel: " + error.message);
    },
  });
};

export const useDeleteTunnel = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteTunnel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tunnels"] });
      toast.success("Tunnel deleted");
    },
    onError: (error) => {
      toast.error("Failed to delete tunnel: " + error.message);
    },
  });
};

export const useConnectTunnel = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: connectTunnel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKeys: ["tunnels"] });
      toast.success("Connected to tunnel");
    },
    onError: (error) => {
      toast.error("Failed to connect to tunnel: " + error.message);
    },
  });
};

export const useDisconnectTunnel = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: disconnectTunnel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKeys: ["tunnels"] });
      toast.success("Disconnected from tunnel");
    },
    onError: (error) => {
      toast.error("Failed to disconnect from tunnel: " + error.message);
    },
  });
};
