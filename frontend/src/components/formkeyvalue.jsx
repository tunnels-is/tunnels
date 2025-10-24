import { Label } from "@/components/ui/label";
import React from "react";

const FormKeyValue = (props) => {
  if (!props?.value) {
    return <></>;
  }

  return (
    <div className="max-w-[400px] py-2">
      <div className="flex mb-1">
        {props?.icon &&
          <props.icon
            className={`h-4 w-4 pb-[1px] text-cyan-500 ${props.iconClass} `} />
        }
        <Label className="ml-1">{props?.label}</Label>
      </div>
      <div>{props?.value}</div>
    </div>
  );
};

export default FormKeyValue;
