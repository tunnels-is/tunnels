import React, { useCallback, useMemo } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Save } from "lucide-react";
import { useAtom, useAtomValue } from "jotai";
import { controlServerAtom, controlServersAtom } from "@/stores/configStore";
import { ButtonGroup } from "@/components/ui/button-group";
import { CopyPlus, Pencil } from "lucide-react";

export const AuthServerEditorDialog = ({
  open,
  onOpenChange,
  object,
  onChange,
  saveButton,
  readOnly = false
}) => {
  if (!object) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>Auth Server</DialogTitle>
        </DialogHeader>
        <div className="grid gap-4 py-4">
          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="id" className="text-right">
              ID
            </Label>
            <Input
              id="id"
              value={object.ID}
              className="col-span-3"
              disabled={true}
            />
          </div>
          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="host" className="text-right">
              Host
            </Label>
            <Input
              id="host"
              value={object.Host || ""}
              onChange={(e) => onChange("Host", e.target.value)}
              className="col-span-3"
              disabled={readOnly}
            />
          </div>

          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="port" className="text-right">
              Port
            </Label>
            <Input
              id="port"
              value={object.Port || ""}
              onChange={(e) => onChange("Port", e.target.value)}
              className="col-span-3"
              disabled={readOnly}
            />
          </div>
          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="https" className="text-right">
              HTTPS
            </Label>
            <div className="flex items-center space-x-2 col-span-3">
              <Switch
                id="https"
                checked={object.HTTPS || false}
                onCheckedChange={(checked) => onChange("HTTPS", checked)}
                disabled={readOnly}
              />
            </div>
          </div>
          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="validate" className="text-right">
              Validate Cert
            </Label>
            <div className="flex items-center space-x-2 col-span-3">
              <Switch
                id="validate"
                checked={object.ValidateCertificate || false}
                onCheckedChange={(checked) => onChange("ValidateCertificate", checked)}
                disabled={readOnly}
              />
            </div>
          </div>
          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="certPath" className="text-right">
              Cert Path
            </Label>
            <Input
              id="certPath"
              value={object.CertificatePath || ""}
              onChange={(e) => onChange("CertificatePath", e.target.value)}
              className="col-span-3"
              disabled={readOnly}
            />
          </div>
        </div>
        <DialogFooter>
          {!readOnly && saveButton && (
            <Button onClick={() => saveButton(object)}>
              <Save className="mr-2 h-4 w-4" />
              Save
            </Button>
          )}
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {readOnly ? "Close" : "Cancel"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export const AuthServerSelect = ({ setModalOpen, setNewAuth }) => {
  const [authServer, setAuthServer] = useAtom(controlServerAtom);
  const controlServers = useAtomValue(controlServersAtom);

  const changeAuthServer = useCallback(
    (id) => {
      controlServers.forEach((s) => {
        if (s.ID === id) setAuthServer(s);
      });
    },
    [controlServers, setAuthServer]
  );

  const opts = useMemo(() => {
    const options = [];
    let tunID = "";
    controlServers.forEach((s) => {
      if (s.Host.includes("api.tunnels.is")) {
        tunID = s.ID;
      }
      options.push({
        value: s.ID,
        key: s.Host + ":" + s.Port,
        selected: s.ID === authServer?.ID,
      });
    });
    return { options, tunID };
  }, [controlServers, authServer?.ID]);

  return (
    <div className="flex items-start">
      <Select
        value={authServer ? authServer.ID : opts.tunID}
        onValueChange={changeAuthServer}
      >
        <SelectTrigger className="w-[320px]">
          <SelectValue placeholder="Select Auth Server" />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {opts.options.map((t) => (
              <SelectItem key={t.value} value={t.value}>
                {t.key}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
      <ButtonGroup className="ml-4 mt-[2px]">
        <Button
          variant="outline"
          size="icon"
          onClick={() => setModalOpen(true)}
        >
          <CopyPlus className="h-4 w-4" />
        </Button>
        <Button
          variant="outline"
          size="icon"
          onClick={() => {
            setNewAuth(authServer);
            setModalOpen(true);
          }}
        >
          <Pencil className="h-4 w-4" />
        </Button>
      </ButtonGroup>
    </div>
  );
};

