import React from "react";
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Save } from "lucide-react";

const AuthServerEditorDialog = ({
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

export default AuthServerEditorDialog;
