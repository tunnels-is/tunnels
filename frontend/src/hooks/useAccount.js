import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { activateLicense } from "../api/account";
import { useSetAtom } from "jotai";
import { userAtom } from "../stores/userStore";
import { getAccounts } from "../api/auth";

export const useActivateLicense = () => {
  const setUser = useSetAtom(userAtom);
  return useMutation({
    mutationFn: activateLicense,
    onSuccess: (data) => {
      setUser(data);
    },
  });
};

export const useGetAccounts = () => {
  const queryClient = useQueryClient();
  return useQuery({
    queryKey: ["accounts"],
    queryFn: () => getAccounts(),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["accounts"] })
  });
};