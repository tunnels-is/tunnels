import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "@/components/ui/input-group";

import {

  Monitor,
} from "lucide-react";
import {
  EnvelopeClosedIcon,
  LockClosedIcon,
  FrameIcon,
} from "@radix-ui/react-icons";

import { Alert, AlertDescription } from "@/components/ui/alert";

export const EmailInput = ({ value, error, onChange }) => (
  <div className="space-y-2">
    <InputGroup className="h-11">
      <InputGroupAddon>
        <EnvelopeClosedIcon className="h-5 w-5 text-[#4B7BF5]" />
      </InputGroupAddon>
      <InputGroupInput
        id="email"
        type="email"
        placeholder="Email / Token"
        value={value || ""}
        name="email"
        onChange={onChange}
      />
    </InputGroup>
    {error && <p className="text-sm text-red-500">{error}</p>}
  </div>
);

export const ConfirmPasswordInput = ({ value, error, onChange }) => (
  <div className="space-y-2">
    <InputGroup className="h-11">
      <InputGroupAddon>
        <LockClosedIcon className="h-5 w-5 text-muted-foreground" />
      </InputGroupAddon>
      <InputGroupInput
        id="password2"
        type="password"
        placeholder="Confirm Password"
        value={value || ""}
        name="password2"
        onChange={onChange}
      />
    </InputGroup>
    {error && <p className="text-sm text-destructive">{error}</p>}
  </div>
);

export const PasswordInput = ({ value, error, onChange }) => (
  <div className="space-y-2">
    <InputGroup className="h-11">
      <InputGroupAddon>
        <LockClosedIcon className="h-5 w-5 text-[#4B7BF5]" />
      </InputGroupAddon>
      <InputGroupInput
        id="password"
        type="password"
        placeholder="Password"
        value={value || ""}
        name="password"
        onChange={onChange}
      />
    </InputGroup>
    {error && <p className="text-sm text-red-500">{error}</p>}
  </div>
);

export const DeviceInput = ({ value, error, onChange }) => (
  <div className="space-y-2">
    <InputGroup className="h-11">
      <InputGroupAddon>
        <Monitor className="h-5 w-5 text-[#4B7BF5]" />
      </InputGroupAddon>
      <InputGroupInput
        id="devicename"
        type="text"
        placeholder="Device Name"
        value={value || ""}
        name="devicename"
        onChange={onChange}
      />
    </InputGroup>
    {error && <p className="text-sm text-red-500">{error}</p>}
  </div>
);

export const TwoFactorInput = ({ value, error, onChange }) => (
  <div className="space-y-2">
    <InputGroup className="h-11">
      <InputGroupAddon>
        <LockClosedIcon className="h-5 w-5 text-[#4B7BF5]" />
      </InputGroupAddon>
      <InputGroupInput
        id="digits"
        type="text"
        placeholder="Two-Factor Auth Code (Optional)"
        value={value || ""}
        name="digits"
        onChange={onChange}
      />
    </InputGroup>
    {error && <p className="text-sm text-red-500">{error}</p>}
  </div>
);

export const TokenInput = ({ value, error, onChange }) => (
  <div className="space-y-2">
    <InputGroup className="h-11">
      <InputGroupAddon>
        <FrameIcon className="h-5 w-5 text-[#4B7BF5]" />
      </InputGroupAddon>
      <InputGroupInput
        id="token"
        type="text"
        placeholder="Token"
        value={value || ""}
        name="email"
        onChange={onChange}
      />
    </InputGroup>
    {value && (
      <Alert variant="destructive" className="mt-2">
        <AlertDescription className="font-semibold">
          SAVE THIS TOKEN!
        </AlertDescription>
      </Alert>
    )}
    {error && <p className="text-sm text-red-500">{error}</p>}
  </div>
);

export const CodeInput = ({ error, onChange }) => (
  <div className="space-y-2">
    <InputGroup className="h-11">
      <InputGroupAddon>
        <FrameIcon className="h-5 w-5 text-[#4B7BF5]" />
      </InputGroupAddon>
      <InputGroupInput
        id="code"
        type="text"
        placeholder="Code"
        name="code"
        onChange={onChange}
      />
    </InputGroup>
    {error && <p className="text-sm text-red-500">{error}</p>}
  </div>
);

export const ResetTwoFactorCodeInput = ({ error, onChange }) => (
  <div className="space-y-2">
    <InputGroup className="h-11">
      <InputGroupAddon>
        <FrameIcon className="h-5 w-5 text-[#4B7BF5]" />
      </InputGroupAddon>
      <InputGroupInput
        id="code"
        type="text"
        placeholder="Reset Code sent in email"
        name="code"
        onChange={onChange}
      />
    </InputGroup>
    {error && <p className="text-sm text-red-500">{error}</p>}
  </div>
);

export const RecoveryInput = ({ value, error, onChange }) => (
  <div className="space-y-2">
    <InputGroup className="h-11">
      <InputGroupAddon>
        <FrameIcon className="h-5 w-5 text-[#4B7BF5]" />
      </InputGroupAddon>
      <InputGroupInput
        id="recovery"
        type="text"
        placeholder="Two Factor Recovery Code"
        value={value || ""}
        name="recovery"
        onChange={onChange}
      />
    </InputGroup>
    {error && <p className="text-sm text-red-500">{error}</p>}
  </div>
);
