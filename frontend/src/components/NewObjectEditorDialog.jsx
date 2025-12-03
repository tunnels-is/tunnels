import React from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Save, X } from "lucide-react";
import NewObjectEditor from "./NewObjectEditor";

import { ScrollArea } from "@/components/ui/scroll-area";

const NewObjectEditorDialog = ({
  open,
  onOpenChange,
  object,
  onChange,
  onArrayChange,
  saveButton,
  opts,
  readOnly = false,
  title,
  description
}) => {
  if (!object) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[600px] bg-[#0B0E14] border-[#1a1f2d] text-white p-0 gap-0 overflow-hidden">
        <DialogHeader className="p-6 pb-4 border-b border-[#1a1f2d] bg-[#0B0E14]">
          <div className="flex items-center justify-between">
            <DialogTitle className="text-xl font-bold">
              {title || "Edit Object"}
            </DialogTitle>
          </div>
          {(description || object?.ID) && (
            <DialogDescription className="text-gray-400">
              {description}
              {object?.ID && <span className="font-mono text-xs ml-2 opacity-50">ID: {object.ID}</span>}
            </DialogDescription>
          )}
        </DialogHeader>

        <ScrollArea className="max-h-[70vh] p-6">
          <NewObjectEditor
            opts={opts}
            obj={object}
            onChange={onChange}
            onArrayChange={onArrayChange}
          />
        </ScrollArea>

        <DialogFooter className="p-6 pt-4 border-t border-[#1a1f2d] bg-[#0B0E14] gap-2">
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {readOnly ? "Close" : "Cancel"}
          </Button>
          {!readOnly && saveButton && (
            <Button
              className="gap-2"
              onClick={() => saveButton(object)}
            >
              <Save className="h-4 w-4" />
              Save Changes
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default NewObjectEditorDialog;
