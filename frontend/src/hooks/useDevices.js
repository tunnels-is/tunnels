import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  getConnectedDevices,
  getDevices,
  deleteDevice,
  updateDevice,
  createDevice,
} from "../api/devices";

export const useConnectedDevices = (controlServer, serverIp) => {
  return useQuery({
    queryKey: ["connectedDevices", controlServer?.Host, serverIp],
    queryFn: () => getConnectedDevices({ controlServer, serverIp }),
    enabled: !!controlServer && !!serverIp,
  });
};

export const useDevices = (offset, limit) => {
  return useQuery({
    queryKey: ["devices", offset, limit],
    queryFn: () => getDevices({ offset, limit }),
  });
};

export const useDeleteDevice = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteDevice,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["devices"] });
    },
  });
};

export const useUpdateDevice = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: updateDevice,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["devices"] });
    },
  });
};

export const useCreateDevice = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createDevice,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["devices"] });
    },
  });
};
