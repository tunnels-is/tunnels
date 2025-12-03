import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getServers, createServer, updateServer, deleteServer } from "../api/servers";
import { toast } from "sonner";

export const useServers = (controlServer) => {
  return useQuery({
    queryKey: ["servers", controlServer?.Host],
    queryFn: () => getServers(controlServer),
    enabled: !!controlServer,
  });
};

export const useCreateServer = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createServer,
    onSuccess: () => {
      queryClient.invalidateQueries(["servers"]);
      toast.success("Server created successfully");
    },
    onError: (error) => {
      toast.error("Failed to create server: " + error.message);
    },
  });
};

export const useUpdateServer = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: updateServer,
    onSuccess: () => {
      queryClient.invalidateQueries(["servers"]);
      toast.success("Server updated successfully");
    },
    onError: (error) => {
      toast.error("Failed to update server: " + error.message);
    },
  });
};

export const useDeleteServer = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteServer,
    onSuccess: () => {
      queryClient.invalidateQueries(["servers"]);
      toast.success("Server deleted successfully");
    },
    onError: (error) => {
      toast.error("Failed to delete server: " + error.message);
    },
  });
};
