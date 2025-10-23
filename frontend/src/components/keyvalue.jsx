import React from "react";

const KeyValue = ({ label, value, defaultValue, className = "" }) => {
  if (!value && !defaultValue) return null;

  return (
    <div
      className={`flex items-start justify-between gap-4 py-2 border-b border-muted ${className}`}
    >
      <div className="text-sm font-medium text-muted-foreground">{label}</div>
      <div className="text-sm text-right break-all text-foreground">
        {value || defaultValue}
      </div>
    </div>
  );
};

export default KeyValue;
