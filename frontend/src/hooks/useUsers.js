import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getUsers, adminUpdateUser } from "../api/users";
import { deleteUser } from "@/api/auth";

export const useUsers = (offset, limit) => {
    return useQuery({
        queryKey: ["users", offset, limit],
        queryFn: () => getUsers({ offset, limit }),
    });
};

export const useAdminUpdateUser = () => {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: adminUpdateUser,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["users"] });
        },
    });
};


export const useDeleteUser = () => {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: deleteUser,
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] })
    });
}
