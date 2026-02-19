import React from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";
import GLOBAL_STATE from "../state";
import { STATE } from "../state";

const ConfirmDialog = () => {
  GLOBAL_STATE("confirm");
  const dialog = STATE.confirmDialog;

  const handleClose = () => {
    STATE.confirmDialog = null;
    STATE.renderPage("confirm");
  };

  const handleConfirm = async () => {
    const method = dialog?.onConfirm;
    handleClose();
    if (method) await method();
  };

  const hasTitle = !!dialog?.title;
  const heading = hasTitle ? dialog.title : dialog?.subtitle;
  const description = hasTitle ? dialog?.subtitle : null;

  const isDestructive = dialog?.subtitle?.toLowerCase().includes("delete") ||
    dialog?.subtitle?.toLowerCase().includes("disconnect");

  return (
    <Dialog open={!!dialog?.open} onOpenChange={(open) => { if (!open) handleClose(); }}>
      <DialogContent className="sm:max-w-[380px] text-white bg-[#0a0d14] border-[#1e2433] p-0 gap-0">
        <div className="px-6 pt-6 pb-4">
          <DialogHeader className="space-y-3">
            <div className="flex items-center gap-3">
              <div className={
                isDestructive
                  ? "w-9 h-9 rounded-full bg-red-500/10 flex items-center justify-center shrink-0"
                  : "w-9 h-9 rounded-full bg-[#4B7BF5]/10 flex items-center justify-center shrink-0"
              }>
                <AlertTriangle className={
                  isDestructive
                    ? "w-4 h-4 text-red-400"
                    : "w-4 h-4 text-[#4B7BF5]"
                } />
              </div>
              <DialogTitle className="text-[15px] font-semibold text-white">
                {heading}
              </DialogTitle>
            </div>
            {description && (
              <DialogDescription className="text-[13px] text-white/50 leading-relaxed pl-12">
                {description}
              </DialogDescription>
            )}
          </DialogHeader>
        </div>

        <DialogFooter className="border-t border-[#1e2433] px-6 py-3 flex-row justify-end gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={handleClose}
            className="text-white/50 hover:text-white hover:bg-white/5"
          >
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={handleConfirm}
            className={
              isDestructive
                ? "bg-red-600 hover:bg-red-500 text-white rounded-md"
                : "bg-[#4B7BF5] hover:bg-[#5d8af7] text-white rounded-md"
            }
          >
            {isDestructive ? "Confirm" : "Yes"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default ConfirmDialog;
