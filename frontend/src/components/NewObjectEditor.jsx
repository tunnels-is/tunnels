import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useState, useCallback, useMemo } from "react";
import { Textarea } from "@/components/ui/textarea";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

// Type detection utility
const getType = (data) => {
  if (Array.isArray(data)) return "array";
  if (data === null) return "null";
  if (typeof data === "object") return "object";
  return typeof data;
};

// Field label component
const FieldLabel = ({ label, isBool = false }) => (
  <Label className={`text-sm font-medium text-gray-200 ${isBool ? "cursor-pointer" : "mb-2 block"}`}>
    {label}
  </Label>
);

// Array field item component
const ArrayFieldItem = ({ value, index, fieldKey, isReadOnly, onArrayChange, onDelete, reload }) => (
  <div className="flex gap-2 items-center">
    <Input
      className="flex-1 bg-[#0B0E14] border-[#1a1f2d]"
      value={value}
      disabled={isReadOnly}
      onChange={(e) => {
        onArrayChange(fieldKey, e.target.value, index);
        reload();
      }}
    />
    {!isReadOnly && (
      <Button
        variant="ghost"
        size="icon"
        className="h-8 w-8 text-red-500 hover:text-red-400 hover:bg-red-950/20"
        onClick={() => {
          onDelete(index);
          reload();
        }}
      >
        <Trash2 className="h-4 w-4" />
      </Button>
    )}
  </div>
);

const NewObjectEditor = ({ obj, opts = {}, onChange, onArrayChange }) => {
  const [trigger, setTrigger] = useState({ id: 1 });

  const reload = useCallback(() => {
    setTrigger((prev) => ({ id: prev.id + 1 }));
  }, []);

  // Label generation
  const getLabel = useCallback((key) => {
    if (opts?.nameMap?.[key]) return opts.nameMap[key];
    if (opts?.nameFormat?.[key]) return opts.nameFormat[key](obj);
    return key;
  }, [opts, obj]);

  // Field visibility and state checks
  const isHidden = useCallback((key) => opts?.fields?.[key] === "hidden", [opts]);
  const isReadOnly = useCallback((key) =>
    opts?.fields?.[key] === "readonly" ||
    ["_id", "CreatedAt", "Added", "UpdatedAt"].includes(key),
    [opts]
  );

  // Array field renderer
  const renderArrayField = useCallback((key) => {
    const items = obj[key] || [];
    const readOnly = isReadOnly(key);

    const handleAddItem = () => {
      const newItem = items.length > 0 ? items[0] : "";
      obj[key].push(newItem);
      reload();
    };

    const handleDeleteItem = (index) => {
      obj[key].splice(index, 1);
      reload();
    };

    return (
      <div key={key} className="space-y-3 pt-2">
        <div className="flex items-center justify-between">
          <FieldLabel label={getLabel(key)} />
          {!readOnly && (
            <Button
              variant="ghost"
              size="sm"
              className="h-6 w-6 p-0 hover:bg-green-900/30 text-green-500"
              onClick={handleAddItem}
            >
              <Plus className="h-4 w-4" />
            </Button>
          )}
        </div>
        <div className="space-y-2 pl-2 border-l-2 border-[#1a1f2d]">
          {items.map((item, i) => (
            <ArrayFieldItem
              key={i}
              value={item}
              index={i}
              fieldKey={key}
              isReadOnly={readOnly}
              onArrayChange={onArrayChange}
              onDelete={handleDeleteItem}
              reload={reload}
            />
          ))}
          {items.length === 0 && (
            <div className="text-xs text-muted-foreground italic">Empty list</div>
          )}
        </div>
      </div>
    );
  }, [obj, getLabel, isReadOnly, onArrayChange, reload]);

  // Boolean field renderer
  const renderBooleanField = useCallback((key) => {
    const label = getLabel(key);
    const readOnly = isReadOnly(key);

    return (
      <div key={key} className="flex items-center justify-between py-2 border-b border-[#1a1f2d]/50 last:border-0">
        <FieldLabel label={label} isBool />
        <Switch
          checked={obj[key]}
          disabled={readOnly}
          onCheckedChange={(checked) => {
            onChange(key, checked, "boolean");
            reload();
          }}
        />
      </div>
    );
  }, [obj, getLabel, isReadOnly, onChange, reload]);

  // Text area field renderer (for PubKey and similar)
  const renderTextAreaField = useCallback((key) => {
    const label = getLabel(key);
    const readOnly = isReadOnly(key);

    return (
      <div key={key} className="space-y-2">
        <FieldLabel label={label} />
        <Textarea
          className="bg-[#0B0E14] border-[#1a1f2d] min-h-[80px] font-mono text-xs"
          value={obj[key]}
          disabled={readOnly}
          onChange={(e) => {
            onChange(key, e.target.value, "string");
            reload();
          }}
        />
      </div>
    );
  }, [obj, getLabel, isReadOnly, onChange, reload]);

  // Standard input field renderer
  const renderInputField = useCallback((key, type) => {
    const label = getLabel(key);
    const readOnly = isReadOnly(key);

    return (
      <div key={key} className="space-y-2">
        <FieldLabel label={label} />
        <Input
          type={type === "number" ? "number" : "text"}
          className="bg-[#0B0E14] border-[#1a1f2d]"
          value={obj[key]}
          disabled={readOnly}
          onChange={(e) => {
            const val = type === "number" ? Number(e.target.value) : e.target.value;
            onChange(key, val, type);
            reload();
          }}
        />
      </div>
    );
  }, [obj, getLabel, isReadOnly, onChange, reload]);

  // Main field renderer
  const renderField = useCallback((key) => {
    if (isHidden(key)) return null;

    const type = getType(obj[key]);

    // Special handling for PubKey field
    if (key === "PubKey") {
      return renderTextAreaField(key);
    }

    // Type-based rendering
    switch (type) {
      case "boolean":
        return renderBooleanField(key);
      case "array":
        return renderArrayField(key);
      case "string":
      case "number":
        return renderInputField(key, type);
      default:
        return null;
    }
  }, [obj, isHidden, renderTextAreaField, renderBooleanField, renderArrayField, renderInputField]);

  // Group fields by type for better layout
  const { boolKeys, otherKeys } = useMemo(() => {
    const keys = Object.keys(obj);
    return {
      boolKeys: keys.filter(k => getType(obj[k]) === "boolean" && !isHidden(k)),
      otherKeys: keys.filter(k => getType(obj[k]) !== "boolean" && !isHidden(k))
    };
  }, [obj, isHidden, trigger]);

  return (
    <div className="space-y-6 p-1">
      {/* Boolean Toggles Group */}
      {boolKeys.length > 0 && (
        <Card className="bg-[#151a25] border-none">
          <CardContent className="p-4 space-y-1">
            {boolKeys.map(k => renderField(k))}
          </CardContent>
        </Card>
      )}

      {/* Other Fields */}
      <div className="space-y-4">
        {otherKeys.map(k => renderField(k))}
      </div>
    </div>
  );
};

export default NewObjectEditor;
