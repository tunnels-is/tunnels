import React, { useEffect, useState, useCallback, useMemo } from "react";
import { useSetAtom, useAtom, useAtomValue } from "jotai";
import {
  userAtom,
  defaultEmailAtom,
  defaultDeviceNameAtom,
} from "../stores/userStore";
import {
  configAtom,
  controlServerAtom,
  controlServersAtom,
} from "../stores/configStore";
import { useSaveControlServer } from "../hooks/useControlServers";
import { v4 as uuidv4 } from "uuid";
import { useNavigate } from "react-router-dom";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "@/components/ui/input-group";
import { ButtonGroup } from "@/components/ui/button-group";
import {
  Edit2 as Edit2Icon,
  CopyPlus as CopyPlusIcon,
  Monitor as DesktopIcon,
  Lock as LockClosedIcon,
  Mail as EnvelopeClosedIcon,
  Frame as FrameIcon,
} from "lucide-react";
import AuthServerEditorDialog from "../components/AuthServerEditorDialog";
import { toast } from "sonner";
import {
  useLoginUser,
  useRegisterUser,
  useEnableUser,
  useResetPassword,
  useSendResetCode,
  useSaveUserToDisk,
} from "../hooks/useAuth";
import { useSaveConfig } from "../hooks/useConfig";

const EmailInput = ({ value, error, onChange }) => (
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

const DeviceInput = ({ value, error, onChange }) => (
  <div className="space-y-2">
    <InputGroup className="h-11">
      <InputGroupAddon>
        <DesktopIcon className="h-5 w-5 text-[#4B7BF5]" />
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

const PasswordInput = ({ value, error, onChange }) => (
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

const TwoFactorInput = ({ value, error, onChange }) => (
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

const ConfirmPasswordInput = ({ value, error, onChange }) => (
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

const TokenInput = ({ value, error, onChange }) => (
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

const CodeInput = ({ error, onChange }) => (
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

const ResetTwoFactorCodeInput = ({ error, onChange }) => (
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

const RecoveryInput = ({ value, error, onChange }) => (
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

const AuthServerSelect = ({ setModalOpen, setNewAuth }) => {
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
          <CopyPlusIcon className="h-4 w-4" />
        </Button>
        <Button
          variant="outline"
          size="icon"
          onClick={() => {
            setNewAuth(authServer);
            setModalOpen(true);
          }}
        >
          <Edit2Icon className="h-4 w-4" />
        </Button>
      </ButtonGroup>
    </div>
  );
};

const LoginForm = ({
  config,
  authServer,
  setModalOpen,
  setNewAuth,
  mode,
  setMode,
}) => {
  const setUser = useSetAtom(userAtom);
  const [defaultEmail, setDefaultEmail] = useAtom(defaultEmailAtom);
  const [defaultDeviceName, setDefaultDeviceName] = useAtom(
    defaultDeviceNameAtom
  );
  const navigate = useNavigate();
  const loginMutation = useLoginUser();
  const saveUserMutation = useSaveUserToDisk();

  const [inputs, setInputs] = useState({
    email: "deiocb@iofn.com",
    password: "1234567897",
    devicename: "windows",
  });
  const [errors, setErrors] = useState({});
  const [remember, setRemember] = useState(false);

  useEffect(() => {
    setInputs((prev) => {
      let i = { ...prev };
      if (defaultDeviceName) i["devicename"] = defaultDeviceName;
      if (defaultEmail) i["email"] = defaultEmail;
      return i;
    });
  }, [defaultDeviceName, defaultEmail]);

  const HandleInputChange = (event) => {
    setInputs((prev) => ({ ...prev, [event.target.name]: event.target.value }));
  };

  const HandleSubmit = async () => {
    let newErrors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      newErrors["email"] = "Email / Token missing";
      hasErrors = true;
    }
    if (!inputs["password"]) {
      newErrors["password"] = "Password missing";
      hasErrors = true;
    }

    // Mode 1: Standard Login
    if (mode === 1) {
      if (!inputs["devicename"]) {
        newErrors["devicename"] = "Device login name missing";
        hasErrors = true;
      }
    }

    // Mode 2: 2FA Login
    if (mode === 2) {
      if (!inputs["digits"] || inputs["digits"].length < 6) {
        newErrors["digits"] = "Code needs to be at least 6 digits";
        hasErrors = true;
      }
    }

    if (hasErrors) {
      setErrors(newErrors);
      return;
    }

    try {
      const data = await loginMutation.mutateAsync(inputs);

      if (data) {
        setDefaultDeviceName(inputs["devicename"]);
        setDefaultEmail(inputs["email"]);
        setUser({ ...data, ControlServer: authServer, ID: data._id });
        if (remember) saveUserMutation.mutate(data);
        navigate("/servers");
      }
    } catch (error) {
      console.error(error);
      // If error indicates 2FA needed, we might switch mode?
    }
    setErrors({});
  };

  return (
    <Card className="w-full max-w-md mx-auto shadow-xl">
      <CardContent className="space-y-6 p-6">
        <EmailInput
          value={inputs["email"]}
          error={errors["email"]}
          onChange={HandleInputChange}
        />
        <DeviceInput
          value={inputs["devicename"]}
          error={errors["devicename"]}
          onChange={HandleInputChange}
        />
        <PasswordInput
          value={inputs["password"]}
          error={errors["password"]}
          onChange={HandleInputChange}
        />
        <TwoFactorInput
          value={inputs["digits"]}
          error={errors["digits"]}
          onChange={HandleInputChange}
        />
        <AuthServerSelect
          config={config}
          setModalOpen={setModalOpen}
          setNewAuth={setNewAuth}
        />
        <Button
          className="w-full h-11 hover:bg-primary/90 text-white"
          onClick={HandleSubmit}
        >
          Login
        </Button>
        <div className="flex items-center space-x-2">
          <Switch checked={remember} onCheckedChange={setRemember} />
          <Label>Remember Login</Label>
        </div>
      </CardContent>
    </Card>
  );
};

const RegisterForm = ({ config, authServer, setModalOpen, setNewAuth }) => {
  const setUser = useSetAtom(userAtom);
  const navigate = useNavigate();
  const registerMutation = useRegisterUser();
  const saveUserMutation = useSaveUserToDisk();

  const [inputs, setInputs] = useState({
    email: "ivfbh@igfn.com",
    password: "1234567897",
    password2: "1234567897",
  });
  const [errors, setErrors] = useState({});
  const [tokenLogin, setTokenLogin] = useState(false);

  const HandleInputChange = (event) => {
    setInputs((prev) => ({ ...prev, [event.target.name]: event.target.value }));
  };

  const GenerateToken = () => {
    setTokenLogin(true);
    setInputs((prev) => ({ ...prev, email: uuidv4() }));
    setErrors({});
  };

  const RemoveToken = () => {
    setTokenLogin(false);
    setInputs((prev) => ({ ...prev, email: "" }));
    setErrors({});
  };

  const RegisterSubmit = async () => {
    let newErrors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      newErrors["email"] = "Email / Token missing";
      hasErrors = true;
    } else {
      if (inputs["email"].length > 320) {
        newErrors["email"] = "Maximum 320 characters";
        hasErrors = true;
      }
      if (
        !tokenLogin &&
        (!inputs["email"].includes(".") || !inputs["email"].includes("@"))
      ) {
        newErrors["email"] = "Invalid email format";
        hasErrors = true;
      }
    }

    if (!inputs["password"]) {
      newErrors["password"] = "Password missing";
      hasErrors = true;
    } else {
      if (inputs["password"].length < 9)
        newErrors["password"] = "Minimum 10 characters";
      if (inputs["password"].length > 255)
        newErrors["password"] = "Maximum 255 characters";
    }

    if (!inputs["password2"]) {
      newErrors["password2"] = "Password confirm missing";
      hasErrors = true;
    } else if (inputs["password"] !== inputs["password2"]) {
      newErrors["password2"] = "Passwords do not match";
      hasErrors = true;
    }

    if (hasErrors) {
      setErrors(newErrors);
      return;
    }

    try {
      const data = await registerMutation.mutateAsync(inputs);
      console.log("User registered", data);
      if (data) {
        data.ControlServer = authServer;
        setUser(data);
        // Assuming remember is false for register as per original UI
        navigate("/servers");
      }
    } catch (error) {
      console.error(error);
    }
    setErrors({});
  };

  return (
    <Card className="w-full max-w-md mx-auto shadow-xl">
      <CardContent className="space-y-6 p-6">
        <div className="text-center mb-2">
          <h1 className="text-lg font-medium text-white/80">
            Create your account
          </h1>
        </div>
        {tokenLogin ? (
          <TokenInput
            value={inputs["email"]}
            error={errors["email"]}
            onChange={HandleInputChange}
          />
        ) : (
          <EmailInput
            value={inputs["email"]}
            error={errors["email"]}
            onChange={HandleInputChange}
          />
        )}
        <PasswordInput
          value={inputs["password"]}
          error={errors["password"]}
          onChange={HandleInputChange}
        />
        <ConfirmPasswordInput
          value={inputs["password2"]}
          error={errors["password2"]}
          onChange={HandleInputChange}
        />
        <AuthServerSelect
          config={config}
          setModalOpen={setModalOpen}
          setNewAuth={setNewAuth}
        />
        <Button
          className="w-full h-11 bg-[#4B7BF5] hover:bg-[#4B7BF5]/90 text-white"
          onClick={RegisterSubmit}
        >
          Register
        </Button>
        <div className="mt-6 text-center">
          <p
            className="text-sm text-muted-foreground cursor-pointer hover:text-primary"
            onClick={() => {
              if (tokenLogin) RemoveToken();
              else GenerateToken();
            }}
          >
            {tokenLogin ? "Register with email" : "Register with token"}
          </p>
        </div>
      </CardContent>
    </Card>
  );
};

const RegisterAnonForm = ({ config, authServer, setModalOpen, setNewAuth }) => {
  const setUser = useSetAtom(userAtom);
  const navigate = useNavigate();
  const registerMutation = useRegisterUser();

  const [inputs, setInputs] = useState({});
  const [errors, setErrors] = useState({});

  const HandleInputChange = (event) => {
    setInputs((prev) => ({ ...prev, [event.target.name]: event.target.value }));
  };

  const RegisterSubmit = async () => {
    // Simplified validation for token only
    let newErrors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      newErrors["email"] = "Token missing";
      hasErrors = true;
    }
    if (!inputs["password"]) {
      newErrors["password"] = "Password missing";
      hasErrors = true;
    }
    if (inputs["password"] !== inputs["password2"]) {
      newErrors["password2"] = "Passwords do not match";
      hasErrors = true;
    }

    if (hasErrors) {
      setErrors(newErrors);
      return;
    }

    try {
      const data = await registerMutation.mutateAsync({
        server: authServer,
        data: inputs,
      });
      if (data) {
        data.ControlServer = authServer;
        setUser(data);
        navigate("/servers");
      }
    } catch (error) {}
    setErrors({});
  };

  return (
    <Card className="w-full max-w-md mx-auto shadow-xl">
      <CardContent className="space-y-6 p-6">
        <div className="text-center mb-2">
          <h1 className="text-lg font-medium text-white/80">
            Anonymous Registration
          </h1>
        </div>
        <Alert className="border-2 border-red-500 bg-red-500/10">
          <AlertDescription className="font-medium text-red-500">
            Save your login token in a secure place, it is the only form of
            authentication you have for your account. If you lose the token your
            account is lost forever.
          </AlertDescription>
        </Alert>
        <TokenInput
          value={inputs["email"]}
          error={errors["email"]}
          onChange={HandleInputChange}
        />
        <PasswordInput
          value={inputs["password"]}
          error={errors["password"]}
          onChange={HandleInputChange}
        />
        <ConfirmPasswordInput
          value={inputs["password2"]}
          error={errors["password2"]}
          onChange={HandleInputChange}
        />
        <AuthServerSelect
          config={config}
          setModalOpen={setModalOpen}
          setNewAuth={setNewAuth}
        />
        <Button
          className="w-full h-11 bg-[#4B7BF5] hover:bg-[#4B7BF5]/90 text-white"
          onClick={RegisterSubmit}
        >
          Register
        </Button>
      </CardContent>
    </Card>
  );
};

const ResetPasswordForm = ({
  config,
  authServer,
  setModalOpen,
  setNewAuth,
  setMode,
}) => {
  const resetPasswordMutation = useResetPassword();
  const sendResetCodeMutation = useSendResetCode();
  const [inputs, setInputs] = useState({});
  const [errors, setErrors] = useState({});

  const HandleInputChange = (event) => {
    setInputs((prev) => ({ ...prev, [event.target.name]: event.target.value }));
  };

  const GetCode = async () => {
    try {
      await sendResetCodeMutation.mutateAsync({ Email: inputs["email"] });
      toast.success("reset code sent");
      setErrors({});
    } catch (error) {}
  };

  const ResetSubmit = async () => {
    let newErrors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      newErrors["email"] = "Email missing";
      hasErrors = true;
    }
    if (!inputs["password"]) {
      newErrors["password"] = "Password missing";
      hasErrors = true;
    }
    if (!inputs["password2"]) {
      newErrors["password2"] = "Confirmation missing";
      hasErrors = true;
    }
    if (inputs["password"] !== inputs["password2"]) {
      newErrors["password"] = "Passwords do not match";
      hasErrors = true;
    }
    if (!inputs["code"]) {
      newErrors["code"] = "Code missing";
      hasErrors = true;
    }

    if (hasErrors) {
      setErrors(newErrors);
      return;
    }

    try {
      await resetPasswordMutation.mutateAsync({
        Email: inputs["email"],
        Password: inputs["password"],
        ResetCode: inputs["code"],
        UseTwoFactor: inputs["usetwofactor"] || false,
      });
      setInputs((prev) => ({ ...prev, password: "", password2: "", code: "" }));
      setMode(1);
    } catch (error) {}
    setErrors({});
  };

  return (
    <Card className="w-full max-w-md mx-auto shadow-xl">
      <CardContent className="space-y-6 p-6">
        <div className="text-center mb-2">
          <h1 className="text-lg font-medium text-white/80">
            Reset your password
          </h1>
        </div>
        <EmailInput
          value={inputs["email"]}
          error={errors["email"]}
          onChange={HandleInputChange}
        />
        <PasswordInput
          value={inputs["password"]}
          error={errors["password"]}
          onChange={HandleInputChange}
        />
        <ConfirmPasswordInput
          value={inputs["password2"]}
          error={errors["password2"]}
          onChange={HandleInputChange}
        />
        <ResetTwoFactorCodeInput
          error={errors["code"]}
          onChange={HandleInputChange}
        />
        <AuthServerSelect
          config={config}
          setModalOpen={setModalOpen}
          setNewAuth={setNewAuth}
        />
        <div className="flex space-x-2">
          <Button
            className="flex-1 h-11 hover:bg-[#4B7BF5]/90 text-white"
            onClick={GetCode}
          >
            Get Code
          </Button>
          <Button
            className="flex-1 h-11 hover:bg-[#4B7BF5]/90 text-white"
            onClick={ResetSubmit}
          >
            Reset Password
          </Button>
        </div>
      </CardContent>
    </Card>
  );
};

const RecoverTwoFactorForm = ({
  config,
  authServer,
  setModalOpen,
  setNewAuth,
}) => {
  const setUser = useSetAtom(userAtom);
  const navigate = useNavigate();
  const loginMutation = useLoginUser();
  const saveUserMutation = useSaveUserToDisk();
  const [inputs, setInputs] = useState({});
  const [errors, setErrors] = useState({});

  const HandleInputChange = (event) => {
    setInputs((prev) => ({ ...prev, [event.target.name]: event.target.value }));
  };

  const HandleSubmit = async () => {
    if (!inputs["recovery"]) {
      setErrors({ recovery: "Recovery code missing" });
      return;
    }
    // Logic similar to login but with recovery code
    try {
      const data = await loginMutation.mutateAsync({
        server: authServer,
        data: inputs,
      });
      if (data) {
        data.ControlServer = authServer;
        setUser(data);
        navigate("/servers");
      }
    } catch (error) {}
  };

  return (
    <Card className="w-full max-w-md mx-auto shadow-xl">
      <CardContent className="space-y-6 p-6">
        <div className="text-center mb-2">
          <h1 className="text-lg font-medium">Two-Factor Recovery</h1>
        </div>
        <EmailInput
          value={inputs["email"]}
          error={errors["email"]}
          onChange={HandleInputChange}
        />
        <PasswordInput
          value={inputs["password"]}
          error={errors["password"]}
          onChange={HandleInputChange}
        />
        <RecoveryInput
          value={inputs["recovery"]}
          error={errors["recovery"]}
          onChange={HandleInputChange}
        />
        <AuthServerSelect setModalOpen={setModalOpen} setNewAuth={setNewAuth} />
        <Button
          className="w-full h-11 bg-[#4B7BF5] hover:bg-[#4B7BF5]/90 text-white"
          onClick={HandleSubmit}
        >
          Login
        </Button>
      </CardContent>
    </Card>
  );
};

const EnableAccountForm = ({
  authServer,
  setModalOpen,
  setNewAuth,
  setMode,
}) => {
  const enableMutation = useEnableUser();
  const [inputs, setInputs] = useState({});
  const [errors, setErrors] = useState({});

  const HandleInputChange = (event) => {
    setInputs((prev) => ({ ...prev, [event.target.name]: event.target.value }));
  };

  const EnableSubmit = async () => {
    if (!inputs["email"] || !inputs["code"]) {
      setErrors({
        email: !inputs["email"] ? "Required" : "",
        code: !inputs["code"] ? "Required" : "",
      });
      return;
    }
    try {
      await enableMutation.mutateAsync({
        server: authServer,
        data: { Email: inputs["email"], ConfirmCode: inputs["code"] },
      });
      setInputs((prev) => ({ ...prev, code: "" }));
      setMode(6); // Stay or move? Original stayed or moved? Original setMode(6) which is self?
    } catch (error) {}
  };

  return (
    <Card className="w-full max-w-md mx-auto shadow-xl">
      <CardContent className="space-y-6 p-6">
        <div className="text-center mb-2">
          <h1 className="text-lg font-medium">Enable your account</h1>
        </div>
        <EmailInput
          value={inputs["email"]}
          error={errors["email"]}
          onChange={HandleInputChange}
        />
        <CodeInput error={errors["code"]} onChange={HandleInputChange} />
        <AuthServerSelect setModalOpen={setModalOpen} setNewAuth={setNewAuth} />
        <Button
          className="w-full h-11 bg-[#4B7BF5] hover:bg-[#4B7BF5]/90 text-white"
          onClick={EnableSubmit}
        >
          Enable Account
        </Button>
      </CardContent>
    </Card>
  );
};

// --- Main Component ---

const Login = (props) => {
  const [authServer, setAuthServer] = useAtom(controlServerAtom);
  const controlServers = useAtomValue(controlServersAtom);
  const saveControlServer = useSaveControlServer();

  const [mode, setMode] = useState(
    props.mode ? Number(props.mode) : props.mode === 0 ? 1 : 1
  );
  const [modalOpen, setModalOpen] = useState(false);
  const [newAuth, setNewAuth] = useState({
    ID: uuidv4(),
    Host: "",
    Port: "",
    HTTPS: true,
    ValidateCertificate: true,
    CertificatePath: "",
  });

  const saveNewAuth = useCallback(() => {
    saveControlServer(newAuth);
    // If we are editing the currently selected server, update the atom
    if (authServer && authServer.ID === newAuth.ID) {
      setAuthServer({ ...newAuth });
    } else if (controlServers.length === 0) {
      // If this is the first server (length 0 before save, but we can't easily know that here without refetching or checking length before save)
      // Actually, controlServers here is the list *before* save.
      setAuthServer({ ...newAuth });
    }
  }, [newAuth, saveControlServer, authServer, setAuthServer, controlServers]);

  useEffect(() => {
    if (controlServers.length > 0 && !authServer) {
      setAuthServer(controlServers[0]);
    }
  }, [controlServers, authServer, setAuthServer]);

  const commonProps = { authServer, setModalOpen, setNewAuth };

  return (
    <div className="min-h-screen w-full flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-md">
        {(mode === 1 || mode === 2) && (
          <LoginForm {...commonProps} mode={mode} setMode={setMode} />
        )}
        {mode === 3 && <RecoverTwoFactorForm {...commonProps} />}
        {mode === 4 && <RegisterForm {...commonProps} />}
        {mode === 5 && <ResetPasswordForm {...commonProps} setMode={setMode} />}
        {mode === 6 && <EnableAccountForm {...commonProps} setMode={setMode} />}
        {mode === 7 && <RegisterAnonForm {...commonProps} />}

        <div className="mt-6 text-center space-y-2">
          {mode === 1 && (
            <>
              <p
                className="text-sm text-muted-foreground cursor-pointer hover:text-primary"
                onClick={() => setMode(4)}
              >
                Create an account
              </p>
              <p
                className="text-sm text-muted-foreground cursor-pointer hover:text-primary"
                onClick={() => setMode(5)}
              >
                Forgot password?
              </p>
              <p
                className="text-sm text-muted-foreground cursor-pointer hover:text-primary"
                onClick={() => setMode(6)}
              >
                Enable account
              </p>
              <p
                className="text-sm text-muted-foreground cursor-pointer hover:text-primary"
                onClick={() => setMode(7)}
              >
                Anonymous Registration
              </p>
            </>
          )}
          {mode !== 1 && (
            <p
              className="text-sm text-muted-foreground cursor-pointer hover:text-primary"
              onClick={() => setMode(1)}
            >
              Back to login
            </p>
          )}
        </div>
      </div>

      <AuthServerEditorDialog
        open={modalOpen}
        onOpenChange={setModalOpen}
        object={newAuth}
        readOnly={false}
        saveButton={() => {
          saveNewAuth();
          setModalOpen(false);
        }}
        onChange={(key, value) =>
          setNewAuth((prev) => ({ ...prev, [key]: value }))
        }
      />
    </div>
  );
};

export default Login;
