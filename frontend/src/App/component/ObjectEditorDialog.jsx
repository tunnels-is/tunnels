import React from "react";
import ObjectEditor from "../ObjectEditor";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Save } from "lucide-react";


const ObjectEditorDialog = ({ 
  open, 
  onOpenChange, 
  object, 
  editorOpts, 
  title = "Edit Object", 
  description = "View or edit object details",
  readOnly = false
}) => {
  const dynamicDescription = object?.Tag 
    ? `${description} for ${object.Tag}`
    : description;
  
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[800px] bg-[#0a0a0a] border-[#222] text-white">
        <DialogHeader>
          <DialogTitle className="text-lg font-bold text-white">{title}</DialogTitle>
          <DialogDescription className="text-white/60">
            {dynamicDescription}
          </DialogDescription>
        </DialogHeader>
        
        {object && (
          <div className="py-4 max-h-[70vh] overflow-y-auto overflow-x-hidden pr-2">
            <ObjectEditor
              opts={editorOpts}
              object={object}
              hideSaveButton={true}
            />
          </div>
        )}
        
        <DialogFooter className="flex items-center justify-end gap-2 pt-4 border-t border-[#222]">
          {!readOnly && editorOpts.saveButton && object && (
            <Button
              variant="outline"
              onClick={() => editorOpts.saveButton(object)}
              className="h-9 border-emerald-800/40 bg-[#0c1e0c] text-emerald-400 hover:bg-emerald-900/30 hover:text-emerald-300 shadow-sm font-medium"
            >
              <Save className="h-4 w-4 mr-1" />
              Save
            </Button>
          )}
          <Button 
            variant="outline" 
            onClick={() => onOpenChange(false)}
            className="h-9 px-4 text-sm font-medium text-white/80 border-[#222] bg-[#111] hover:bg-[#222] hover:text-white"
          >
            {readOnly ? "Close" : "Cancel"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default ObjectEditorDialog; 