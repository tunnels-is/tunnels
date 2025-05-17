import React from "react";
import ObjectEditor from "./ObjectEditor";
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
import NewObjectEditor from "./NewObjectEdior";
import GLOBAL_STATE from "../state";


const NewObjectEditorDialog = ({
  open,
  onOpenChange,
  object,
  onChange,
  saveButton,
  title = "Edit Object",
  description = "View or edit object details",
  readOnly = false
}) => {
  const state = GLOBAL_STATE("editor-dialog");
  const dynamicDescription = object?.Tag
    ? `${description} ${object.Tag}`
    : description;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={"sm:max-w-[800px] text-white" + state.Theme?.menuBG + state.Theme?.borderColor} >
        <DialogHeader>
          <DialogTitle className="text-lg font-bold text-white">{dynamicDescription}</DialogTitle>
        </DialogHeader>

        {object && (
          <div className=" max-h-[70vh] overflow-y-auto overflow-x-hidden pr-2">
            <NewObjectEditor
              obj={object}
              onChange={onChange}
            />
          </div>
        )}

        <DialogFooter className="flex gap-2 mt-2 ">
          {!readOnly && saveButton && object && (
            <Button
              variant="outline"
              className={state.Theme?.successBtn}
              onClick={() => saveButton(object)}
            >
              <Save className="h-4 w-4 mr-1" />
              Save
            </Button>
          )}
          <Button
            variant="outline"
            className={state.Theme?.warningBtn}
            onClick={() => onOpenChange(false)}
          >
            {readOnly ? "Close" : "Cancel"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog >
  );
};

export default NewObjectEditorDialog; 
