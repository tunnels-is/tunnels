import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getUsers, updateUser, deleteUser } from "../api/users";

export const useUsers = (offset, limit) => {
  return useQuery({
    queryKey: ["users", offset, limit],
    queryFn: () => getUsers({ offset, limit }),
  });
};


export const useDeleteUser = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteUser,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] })
  });
}

export const useGetUsers = ({ offset, limit }) => {
  const queryClient = useQueryClient();
  return useQuery({
    queryKey: ["users", offset, limit],
    queryFn: () => getUsers({ offset, limit }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users", offset, limit] })
  });
}


export const useUpdateUser = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: updateUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}