import { useMutation } from "@tanstack/react-query";
import { loginUser, registerUser, enableUser, resetPassword, sendResetCode, setUser } from "../api/auth";
import { toast } from "sonner";

export const useLoginUser = () => {
  return useMutation({
    mutationFn: (data) => loginUser(data),
    onError: (error) => {
      console.error("Login failed", error);
    }
  });
};

export const useRegisterUser = () => {
  return useMutation({
    mutationFn: (data) => registerUser(data),
    onError: (err) => toast.error(err.message)
  });
};

export const useEnableUser = () => {
  return useMutation({
    mutationFn: ({ server, data }) => enableUser(data),
  });
};

export const useResetPassword = () => {
  return useMutation({
    mutationFn: (data) => resetPassword(data),
  });
};

export const useSendResetCode = () => {
  return useMutation({
    mutationFn: (data) => sendResetCode(data),
  });
};

export const useSetUser = () => {
  return useMutation({
    mutationFn: (user) => setUser(user),
    onError: () => {
      toast.error("Unable to switch to target account");
    }
  });
}