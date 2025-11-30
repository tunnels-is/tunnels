import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
    getGroups,
    getGroup,
    createGroup,
    updateGroup,
    deleteGroup,
    addEntityToGroup,
    removeEntityFromGroup,
    getGroupEntities,
} from "../api/groups";

export const useGroups = (offset, limit) => {
    return useQuery({
        queryKey: ["groups", offset, limit],
        queryFn: () => getGroups({ offset, limit }),
    });
};

export const useGroup = (id) => {
    return useQuery({
        queryKey: ["group", id],
        queryFn: () => getGroup(id),
        enabled: !!id,
    });
};

export const useCreateGroup = () => {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: createGroup,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["groups"] });
        },
    });
};

export const useUpdateGroup = () => {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: updateGroup,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["groups"] });
            queryClient.invalidateQueries({ queryKey: ["group"] });
        },
    });
};

export const useDeleteGroup = () => {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: deleteGroup,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["groups"] });
        },
    });
};

export const useAddEntityToGroup = () => {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: addEntityToGroup,
        onSuccess: (_, variables) => {
            queryClient.invalidateQueries({ queryKey: ["groupEntities", variables.groupId, variables.type] });
        },
    });
};

export const useRemoveEntityFromGroup = () => {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: removeEntityFromGroup,
        onSuccess: (_, variables) => {
            queryClient.invalidateQueries({ queryKey: ["groupEntities", variables.groupId, variables.type] });
        },
    });
};

export const useGroupEntities = (groupId, type, offset, limit) => {
    return useQuery({
        queryKey: ["groupEntities", groupId, type, offset, limit],
        queryFn: () => getGroupEntities({ groupId, type, offset, limit }),
        enabled: !!groupId && !!type,
    });
};
