import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import React, { useEffect } from "react";

const FormKeyValue = (props) => {
  if (!props?.value) {
    return <></>;
  }

  return (
    <div className="max-w-[400px] py-2">
      <Label className="text-white">{props?.label}</Label>

      <div className="text-white">{props?.value}</div>
    </div>
  );
};

export default FormKeyValue;
