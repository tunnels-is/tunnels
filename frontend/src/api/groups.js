import { forwardToController } from "./client";

export const getGroups = async ({ offset, limit }) => {
    const response = await forwardToController("POST", "/v3/group/list", { Offset: offset, Limit: limit }, true);
    return response;
};

export const getGroup = async (id) => {
    const response = await forwardToController("POST", "/v3/group", { GID: id }, true);
    return response;
};

export const createGroup = async (group) => {
    const response = await forwardToController("POST", "/v3/group/create", { Group: group }, true);
    return response;
};

export const updateGroup = async (group) => {
    const response = await forwardToController("POST", "/v3/group/update", { Group: group }, true);
    return response;
};

export const deleteGroup = async (id) => {
    const response = await forwardToController("POST", "/v3/group/delete", { GID: id }, true);
    return response;
};

export const addEntityToGroup = async ({ groupId, typeId, type, typeTag }) => {
    const response = await forwardToController("POST", "/v3/group/add", {
        GroupID: groupId,
        TypeID: typeId,
        Type: type,
        TypeTag: typeTag,
    }, true);
    return response;
};

export const removeEntityFromGroup = async ({ groupId, typeId, type }) => {
    const response = await forwardToController("POST", "/v3/group/remove", {
        GroupID: groupId,
        TypeID: typeId,
        Type: type,
    }, true);
    return response;
};

export const getGroupEntities = async ({ groupId, type, offset, limit }) => {
    const response = await forwardToController("POST", "/v3/group/entities", {
        GID: groupId,
        Type: type,
        Limit: limit,
        Offset: offset,
    }, true);
    return response;
};
