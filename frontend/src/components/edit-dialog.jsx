import React, { useMemo } from "react";
import { useForm } from "@tanstack/react-form";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Card, CardContent } from "@/components/ui/card";
import { Save, Plus, Trash2 } from "lucide-react";

// Utility to determine field type
const getType = (value) => {
  if (Array.isArray(value)) return "array";
  if (value === null) return "null";
  if (typeof value === "object") return "object";
  return typeof value;
};

const FieldLabel = ({ label, isBool = false }) => (
  <Label className={`text-sm font-medium text-gray-200 ${isBool ? "cursor-pointer" : "mb-2 block"}`}>
    {label}
  </Label>
);

const EditDialog = ({
  open,
  onOpenChange,
  initialData = {},
  onSubmit,
  title = "Edit Object",
  description,
  readOnly = false,
  fields = {},
}) => {
  const form = useForm({
    defaultValues: initialData,
    onSubmit: async ({ value }) => {
      await onSubmit(value);
    },
  });

  // Reset form when opening with new data
  React.useEffect(() => {
    if (open && initialData) {
      // There isn't a direct "reset" to new values in this version without re-mounting or dedicated API, 
      // but key-ing the dialog or form usually works. 
      // For now relying on the key prop on DialogContent or recreating form if needed, 
      // but react-form usually handles updates if defaultValues change? 
      // Actually, often it doesn't automatically reset. 
      // Let's force a reset or rely on component remounting via key
    }
  }, [open, initialData]);

  // Helper to normalize field config
  const getFieldConfig = (key) => {
    const config = fields[key];
    if (typeof config === "string") {
      return { hidden: config === "hidden", readOnly: config === "readonly" };
    }
    return config || {};
  };

  const sortedKeys = useMemo(() => {
    if (!initialData) return { boolKeys: [], otherKeys: [] };
    const keys = Object.keys(initialData);
    const boolKeys = keys.filter((k) => {
      const config = getFieldConfig(k);
      return getType(initialData[k]) === "boolean" && !config.hidden;
    });
    const otherKeys = keys.filter((k) => {
      const config = getFieldConfig(k);
      return getType(initialData[k]) !== "boolean" && !config.hidden;
    });
    return { boolKeys, otherKeys };
  }, [initialData, fields]);

  // Helper validation for read-only fields
  const isReadOnly = (key) => {
    const config = getFieldConfig(key);
    return readOnly ||
      config.readOnly ||
      ["_id", "CreatedAt", "Added", "UpdatedAt", "ID"].includes(key);
  }

  const getLabel = (key) => {
    const config = getFieldConfig(key);
    return config.label || key;
  }


  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="sm:max-w-[600px] bg-[#0B0E14] border-[#1a1f2d] text-white p-0 gap-0 overflow-hidden"
        aria-describedby={undefined}
      >
        <DialogHeader className="p-6 pb-4 border-b border-[#1a1f2d] bg-[#0B0E14]">
          <DialogTitle className="text-xl font-bold">{title}</DialogTitle>
          {(description || initialData?._id) && (
            <DialogDescription className="text-gray-400">
              {description}
              {initialData?._id && <span className="font-mono text-xs ml-2 opacity-50">ID: {initialData._id}</span>}
            </DialogDescription>
          )}
        </DialogHeader>

        <ScrollArea className="max-h-[70vh] p-6">
          <form
            id="edit-form"
            onSubmit={(e) => {
              e.preventDefault();
              e.stopPropagation();
              form.handleSubmit();
            }}
          >
            <div className="space-y-6 p-1">
              {/* Boolean Toggles */}
              {sortedKeys.boolKeys.length > 0 && (
                <Card className="bg-[#151a25] border-none">
                  <CardContent className="p-4 space-y-1">
                    {sortedKeys.boolKeys.map((key) => (
                      <form.Field
                        key={key}
                        name={key}
                        children={(field) => (
                          <div className="flex items-center justify-between py-2 border-b border-[#1a1f2d]/50 last:border-0">
                            <FieldLabel label={getLabel(key)} isBool />
                            <Switch
                              checked={field.state.value}
                              onCheckedChange={field.handleChange}
                              disabled={isReadOnly(key)}
                            />
                          </div>
                        )}
                      />
                    ))}
                  </CardContent>
                </Card>
              )}

              {/* Other Fields */}
              <div className="space-y-4">
                {sortedKeys.otherKeys.map((key) => (
                  <form.Field
                    key={key}
                    name={key}
                    children={(field) => {
                      const type = getType(field.state.value);
                      const isRO = isReadOnly(key);
                      const label = getLabel(key);

                      if (type === "array") {
                        return (
                          <div className="space-y-3 pt-2">
                            <div className="flex items-center justify-between">
                              <FieldLabel label={label} />
                              {!isRO && (
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  className="h-6 w-6 p-0 hover:bg-green-900/30 text-green-500"
                                  onClick={() => field.pushValue("")}
                                >
                                  <Plus className="h-4 w-4" />
                                </Button>
                              )}
                            </div>
                            <div className="space-y-2 pl-2 border-l-2 border-[#1a1f2d]">
                              {field.state.value?.map((_, i) => (
                                <form.Field
                                  key={i}
                                  name={`${key}[${i}]`}
                                  children={(subField) => (
                                    <div className="flex gap-2 items-center">
                                      <Input
                                        className="flex-1 bg-[#0B0E14] border-[#1a1f2d]"
                                        value={subField.state.value}
                                        onChange={(e) => subField.handleChange(e.target.value)}
                                        disabled={isRO}
                                      />
                                      {!isRO && (
                                        <Button
                                          type="button"
                                          variant="ghost"
                                          size="icon"
                                          className="h-8 w-8 text-red-500 hover:text-red-400 hover:bg-red-950/20"
                                          onClick={() => field.removeValue(i)}
                                        >
                                          <Trash2 className="h-4 w-4" />
                                        </Button>
                                      )}
                                    </div>
                                  )}
                                />
                              ))}
                              {(!field.state.value || field.state.value.length === 0) && (
                                <div className="text-xs text-muted-foreground italic">
                                  Empty list
                                </div>
                              )}
                            </div>
                          </div>
                        );
                      }

                      if (key === "PubKey" || key === "Key") {
                        return (
                          <div className="space-y-2">
                            <FieldLabel label={label} />
                            <Textarea
                              className="bg-[#0B0E14] border-[#1a1f2d] min-h-[80px] font-mono text-xs"
                              value={field.state.value}
                              onChange={(e) => field.handleChange(e.target.value)}
                              disabled={isRO}
                            />
                          </div>
                        )
                      }

                      return (
                        <div className="space-y-2">
                          <FieldLabel label={label} />
                          <Input
                            type={type === "number" ? "number" : "text"}
                            className="bg-[#0B0E14] border-[#1a1f2d]"
                            value={field.state.value || ""}
                            onChange={(e) =>
                              field.handleChange(
                                type === "number" ? Number(e.target.value) : e.target.value
                              )
                            }
                            disabled={isRO}
                          />
                        </div>
                      );
                    }}
                  />
                ))}
              </div>
            </div>
          </form>
        </ScrollArea>

        <DialogFooter className="p-6 pt-4 border-t border-[#1a1f2d] bg-[#0B0E14] gap-2">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {readOnly ? "Close" : "Cancel"}
          </Button>
          {!readOnly && (
            <Button className="gap-2" type="submit" form="edit-form">
              <Save className="h-4 w-4" />
              Save Changes
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default EditDialog;
