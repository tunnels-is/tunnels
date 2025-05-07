import { Form, FormLabel } from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { useState } from "react"

const NewObjectEditor = (props) => {
  const [trigger, setTrigger] = useState({ id: 1 });
  const reload = () => {
    let xx = { ...trigger };
    xx.id += 1;
    setTrigger(xx);
  };

  return (
    <Form>
      {Object.keys(props.obj).map(k => {
        let type = getType(props.obj[k])
        if (type === "array" || type === "object") {
          return
        }

        if (type === "boolean") {
          return (
            <div key={k} className="mt-4">
              <Label className="text-stone-400" type="bool" >{k}</Label>
              <Switch className="ml-2 text-stone-100 hover:border-blue-300"
                value={props.obj[k]}
                onCheckedChange={(e) => {
                  props.onChange(k, Boolean(e), type)
                  reload()
                }}
              />
            </div>
          )
        }

        if (type === "string" || type === "number") {
          return (
            <div key={k} className="">
              <Label className="text-stone-400" >{k}</Label>
              <Input className="w-[400px] text-stone-100 hover:border-blue-300" onChange={(e) => {
                if (type === "number") {
                  props.onChange(k, Number(e.target.value), type)
                } else {
                  props.onChange(k, e.target.value, type)
                }
                reload()
              }} type={type} value={props.obj[k]} />
            </div>
          )
        }

      })}
    </Form >
  )
}

const getType = (data) => {
  switch (Object.prototype.toString.call(data)) {
    case "[object Array]":
      return "array";
    case "[object Object]":
      return "object";
    default:
      let to = typeof data;
      switch (to) {
        case "boolean":
          return "boolean";
        case "number":
          return "number";
        default:
          return "string";
      }
  }
};

export default NewObjectEditor
