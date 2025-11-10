import React from "react";
import {
  Dialog,
  DialogContent,
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
  onArrayChange,
  saveButton,
  opts,
  readOnly = false
}) => {
  const state = GLOBAL_STATE("editor-dialog");
  if (!object) {
    return
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={"sm:max-w-[800px] text-white" + state.Theme?.menuBG + state.Theme?.borderColor} >
        <DialogHeader>
          {object?.Tag &&
            <DialogTitle className="text-lg font-bold text-white">{object?.Tag}</DialogTitle>
          }
          {object?._id &&
            <DialogTitle className="text-lg font-bold text-white">id: {object?._id}</DialogTitle>
          }
        </DialogHeader>

        {object && (
          <div className=" max-h-[70vh] overflow-y-auto overflow-x-hidden pr-2">
            <NewObjectEditor
              opts={opts}
              obj={object}
              onChange={onChange}
              onArrayChange={onArrayChange}
            />
          </div>
        )}

        <DialogFooter className="flex gap-2 mt-2 ">
          {!readOnly && saveButton && object && (
            <Button
              className={state.Theme?.successBtn}
              onClick={() => saveButton(object)}
            >
              <Save className="h-4 w-4 mr-1" />
              Save
            </Button>
          )}
          <Button
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
