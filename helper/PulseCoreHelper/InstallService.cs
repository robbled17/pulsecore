using System;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Security.Principal;
using System.ServiceProcess;
using System.Text;

namespace PulseCoreHelper
{
    internal static class InstallService
    {
        internal static bool IsAdministrator()
        {
            using (var id = WindowsIdentity.GetCurrent())
            {
                var principal = new WindowsPrincipal(id);
                return principal.IsInRole(WindowsBuiltInRole.Administrator);
            }
        }

        internal static string CurrentUserSid()
        {
            using (var id = WindowsIdentity.GetCurrent())
            {
                return id.User != null ? id.User.Value : "";
            }
        }

        internal static string CurrentUserName()
        {
            using (var id = WindowsIdentity.GetCurrent())
            {
                return string.IsNullOrEmpty(id.Name) ? Environment.UserName : id.Name;
            }
        }

        internal static string ExePath()
        {
            var loc = Assembly.GetExecutingAssembly().Location;
            if (string.IsNullOrEmpty(loc))
                loc = Process.GetCurrentProcess().MainModule != null
                    ? Process.GetCurrentProcess().MainModule.FileName
                    : "";
            return Path.GetFullPath(loc);
        }

        internal static int Install(string[] args)
        {
            if (!IsAdministrator())
            {
                HelperLog.Error("install requires elevation");
                Console.Error.WriteLine("PulseCoreHelper install requires Administrator privileges.");
                return 2;
            }

            var pawnioSetup = FindArgValue(args, "-pawnio");
            var srcDir = FindArgValue(args, "-src");
            var pawnioOwned = string.Equals(
                Environment.GetEnvironmentVariable("PULSECORE_PAWNIO_OWNED"),
                "1",
                StringComparison.Ordinal);
            if (!string.IsNullOrEmpty(pawnioSetup))
            {
                if (IsPawnIOServicePresent())
                {
                    HelperLog.Info("PawnIO service already present - skipping setup");
                }
                else
                {
                    var code = InstallPawnIO(pawnioSetup);
                    if (code != 0 && code != 3010) // 3010 = reboot required
                    {
                        HelperLog.Error("PawnIO setup failed exit=" + code);
                        Console.Error.WriteLine("PawnIO setup failed with exit code " + code);
                        WriteInstallDone(code);
                        return code;
                    }
                }
                pawnioOwned = true;
                Environment.SetEnvironmentVariable("PULSECORE_PAWNIO_OWNED", "1");
            }

            // Prefer explicit -src; otherwise use the directory of this elevated bootstrap exe.
            if (string.IsNullOrWhiteSpace(srcDir))
                srcDir = Path.GetDirectoryName(ExePath()) ?? "";

            StopRunningHelpers();
            try { File.Delete(HelperPaths.ReadyMarkerPath); } catch { }

            var exe = SecureInstall.DeployFromSource(srcDir);
            if (!SecureInstall.VerifyInstalledIntegrity(out var integReason))
            {
                HelperLog.Error("Integrity check failed after deploy: " + integReason);
                Console.Error.WriteLine("Helper integrity check failed: " + integReason);
                WriteInstallDone(3);
                return 3;
            }

            var user = CurrentUserName();
            var sid = CurrentUserSid();
            HelperLog.Info("Installing scheduled task " + HelperPaths.TaskName + " for " + user + " exe=" + exe);

            Exception? comEx = null;
            try
            {
                RegisterTaskCom(exe, user);
            }
            catch (Exception ex)
            {
                comEx = ex;
                HelperLog.Warn("COM task register failed, falling back to schtasks: " + ex.Message);
                RegisterTaskSchtasks(exe, user);
            }

            TrySetRestartOnFailure();

            var hash = SecureInstall.Sha256File(exe);
            var state = new HelperState
            {
                UserSid = sid,
                InstalledByPulseCore = true,
                Enabled = true,
                PawnioInstalledByUs = pawnioOwned,
                TaskName = HelperPaths.TaskName,
                HelperPath = exe,
                HelperVersion = SecureInstall.HelperVersion,
                ProtocolVersion = SecureInstall.ProtocolVersion,
                HelperSha256 = hash,
                InstalledAt = DateTime.UtcNow.ToString("o")
            };
            StateStore.Save(state);
            HelperLog.Info("Wrote helper-state.json sid=" + sid + " path=" + exe);

            StartTask();

            // Wait until the serving instance is up (pipe or ready marker).
            var deadline = DateTime.UtcNow.AddSeconds(8);
            while (DateTime.UtcNow < deadline)
            {
                if (File.Exists(HelperPaths.ReadyMarkerPath))
                    break;
                if (Process.GetProcessesByName("PulseCoreHelper").Length > 0)
                {
                    // Give pipe a moment after process start.
                    System.Threading.Thread.Sleep(400);
                    break;
                }
                System.Threading.Thread.Sleep(200);
            }

            try
            {
                File.WriteAllText(HelperPaths.ReadyMarkerPath, DateTime.UtcNow.ToString("o"));
            }
            catch (Exception ex)
            {
                HelperLog.Warn("ready marker: " + ex.Message);
            }

            Console.WriteLine("PulseCoreHelper installed (task " + HelperPaths.TaskName + ").");
            Console.WriteLine("EXE=" + exe);
            Console.WriteLine("SHA256=" + hash);
            if (comEx != null)
                Console.WriteLine("Note: used schtasks fallback after COM error: " + comEx.Message);
            WriteInstallDone(0);
            return 0;
        }

        private static void WriteInstallDone(int code)
        {
            try
            {
                HelperPaths.EnsureDataDir();
                File.WriteAllText(
                    Path.Combine(HelperPaths.DataDir, "install.done"),
                    code.ToString() + Environment.NewLine + DateTime.UtcNow.ToString("o"));
            }
            catch (Exception ex)
            {
                HelperLog.Warn("install.done: " + ex.Message);
            }
        }

        internal static int Uninstall()
        {
            if (!IsAdministrator())
            {
                HelperLog.Error("uninstall requires elevation");
                Console.Error.WriteLine("PulseCoreHelper uninstall requires Administrator privileges.");
                return 2;
            }

            HelperLog.Info("Uninstalling scheduled task " + HelperPaths.TaskName);
            try
            {
                RunCaptured("schtasks", "/End /TN \"" + HelperPaths.TaskName + "\"");
            }
            catch (Exception ex)
            {
                HelperLog.Warn("schtasks /End: " + ex.Message);
            }

            StopRunningHelpers();
            DeleteTask();
            SecureInstall.RemoveProgramFilesHelper();
            try { File.Delete(HelperPaths.ReadyMarkerPath); } catch { }
            try { File.Delete(HelperPaths.StatePath); } catch { }

            Console.WriteLine("PulseCoreHelper uninstalled.");
            WriteInstallDone(0);
            return 0;
        }

        private static string FindArgValue(string[] args, string name)
        {
            if (args == null)
                return "";
            for (var i = 0; i < args.Length - 1; i++)
            {
                if (string.Equals(args[i], name, StringComparison.OrdinalIgnoreCase))
                    return args[i + 1] ?? "";
            }
            return "";
        }

        private static bool IsPawnIOServicePresent()
        {
            try
            {
                using (var sc = new ServiceController("PawnIO"))
                {
                    var _ = sc.Status;
                    return true;
                }
            }
            catch
            {
                return false;
            }
        }

        private static bool IsPawnIOServiceRunning()
        {
            try
            {
                using (var sc = new ServiceController("PawnIO"))
                {
                    return sc.Status == ServiceControllerStatus.Running;
                }
            }
            catch
            {
                return false;
            }
        }

        private static int InstallPawnIO(string setupPath)
        {
            if (string.IsNullOrWhiteSpace(setupPath) || !File.Exists(setupPath))
            {
                HelperLog.Error("PawnIO setup missing: " + setupPath);
                return 2;
            }

            if (IsPawnIOServicePresent())
            {
                HelperLog.Info("PawnIO already installed (RUNNING=" + IsPawnIOServiceRunning() + ") - skip");
                return 0;
            }

            // Official PawnIO.Setup requires -install or -uninstall (not -silent alone).
            HelperLog.Info("Installing PawnIO via official setup: " + setupPath + " -install");
            var psi = new ProcessStartInfo
            {
                FileName = setupPath,
                Arguments = "-install",
                UseShellExecute = false,
                CreateNoWindow = true
            };
            using (var p = Process.Start(psi))
            {
                if (p == null)
                    return 1;
                p.WaitForExit(120000);
                HelperLog.Info("PawnIO setup exit=" + p.ExitCode);
                return p.ExitCode;
            }
        }

        private static int UninstallPawnIO(string setupPath)
        {
            if (string.IsNullOrWhiteSpace(setupPath) || !File.Exists(setupPath))
            {
                var fallback = @"C:\Program Files\PawnIO\uninstall.exe";
                if (File.Exists(fallback))
                    setupPath = fallback;
                else
                    return 2;
            }
            HelperLog.Info("Uninstalling PawnIO: " + setupPath + " -uninstall");
            var psi = new ProcessStartInfo
            {
                FileName = setupPath,
                Arguments = "-uninstall",
                UseShellExecute = false,
                CreateNoWindow = true
            };
            using (var p = Process.Start(psi))
            {
                if (p == null)
                    return 1;
                p.WaitForExit(120000);
                return p.ExitCode;
            }
        }

        internal static void StopRunningHelpers()
        {
            var self = Process.GetCurrentProcess().Id;
            foreach (var p in Process.GetProcessesByName("PulseCoreHelper"))
            {
                try
                {
                    if (p.Id == self)
                        continue;
                    HelperLog.Info("Stopping helper pid=" + p.Id);
                    p.Kill();
                    p.WaitForExit(4000);
                }
                catch (Exception ex)
                {
                    HelperLog.Warn("Failed stopping pid " + p.Id + ": " + ex.Message);
                }
                finally
                {
                    p.Dispose();
                }
            }
        }

        private static void StartTask()
        {
            try
            {
                var r = RunCaptured("schtasks", "/Run /TN \"" + HelperPaths.TaskName + "\"");
                HelperLog.Info("schtasks /Run exit=" + r.ExitCode + " " + r.Output);
                if (r.ExitCode != 0)
                    HelperLog.Warn("Task start returned " + r.ExitCode);
            }
            catch (Exception ex)
            {
                HelperLog.Error("Failed to start task: " + ex.Message);
                throw;
            }
        }

        private static void DeleteTask()
        {
            try
            {
                var r = RunCaptured("schtasks", "/Delete /TN \"" + HelperPaths.TaskName + "\" /F");
                HelperLog.Info("schtasks /Delete exit=" + r.ExitCode + " " + r.Output);
            }
            catch (Exception ex)
            {
                HelperLog.Warn("schtasks /Delete: " + ex.Message);
            }

            try
            {
                var type = Type.GetTypeFromProgID("Schedule.Service");
                if (type == null)
                    return;
                dynamic service = Activator.CreateInstance(type);
                service.Connect();
                dynamic folder = service.GetFolder("\\");
                folder.DeleteTask(HelperPaths.TaskName, 0);
            }
            catch
            {
                // Already removed via schtasks, or never existed.
            }
        }

        private static void RegisterTaskCom(string exe, string user)
        {
            var type = Type.GetTypeFromProgID("Schedule.Service");
            if (type == null)
                throw new InvalidOperationException("Schedule.Service ProgID missing");

            dynamic service = Activator.CreateInstance(type);
            service.Connect();
            dynamic folder = service.GetFolder("\\");
            dynamic def = service.NewTask(0);

            def.RegistrationInfo.Description = "PulseCore elevated hardware helper (named pipe PulseCoreHelper)";
            def.Settings.Enabled = true;
            def.Settings.Hidden = false;
            def.Settings.AllowDemandStart = true;
            def.Settings.StartWhenAvailable = true;
            def.Settings.DisallowStartIfOnBatteries = false;
            def.Settings.StopIfGoingOnBatteries = false;
            def.Settings.ExecutionTimeLimit = "PT0S";
            def.Settings.RestartCount = 3;
            def.Settings.RestartInterval = "PT1M";
            def.Settings.MultipleInstances = 3; // TASK_INSTANCES_STOP_EXISTING
            def.Settings.IdleSettings.StopOnIdleEnd = false;

            def.Principal.UserId = user;
            def.Principal.LogonType = 3; // TASK_LOGON_INTERACTIVE_TOKEN
            def.Principal.RunLevel = 1; // TASK_RUNLEVEL_HIGHEST

            dynamic trigger = def.Triggers.Create(9); // TASK_TRIGGER_LOGON
            trigger.Enabled = true;
            trigger.UserId = user;

            dynamic action = def.Actions.Create(0); // TASK_ACTION_EXEC
            action.Path = exe;
            action.WorkingDirectory = Path.GetDirectoryName(exe) ?? "";

            // TASK_CREATE_OR_UPDATE = 6, TASK_LOGON_INTERACTIVE_TOKEN = 3
            folder.RegisterTaskDefinition(
                HelperPaths.TaskName,
                def,
                6,
                user,
                Type.Missing,
                3);
            HelperLog.Info("Registered scheduled task via COM");
        }

        private static void RegisterTaskSchtasks(string exe, string user)
        {
            var tr = "\"" + exe + "\"";
            var args = "/Create /TN \"" + HelperPaths.TaskName + "\" /SC ONLOGON /RL HIGHEST /F /IT /RU \"" +
                       user + "\" /TR " + tr;
            var r = RunCaptured("schtasks", args);
            HelperLog.Info("schtasks /Create exit=" + r.ExitCode + " " + r.Output);
            if (r.ExitCode != 0)
                throw new InvalidOperationException("schtasks create failed: " + r.Output);
        }

        private static void TrySetRestartOnFailure()
        {
            try
            {
                var type = Type.GetTypeFromProgID("Schedule.Service");
                if (type == null)
                    return;
                dynamic service = Activator.CreateInstance(type);
                service.Connect();
                dynamic folder = service.GetFolder("\\");
                dynamic task = folder.GetTask(HelperPaths.TaskName);
                dynamic def = task.Definition;
                def.Settings.RestartCount = 3;
                def.Settings.RestartInterval = "PT1M";
                def.Settings.ExecutionTimeLimit = "PT0S";
                def.Settings.DisallowStartIfOnBatteries = false;
                def.Settings.StopIfGoingOnBatteries = false;
                folder.RegisterTaskDefinition(
                    HelperPaths.TaskName,
                    def,
                    4, // TASK_UPDATE
                    Type.Missing,
                    Type.Missing,
                    task.Definition.Principal.LogonType);
                HelperLog.Info("Set task restart-on-failure (3 x 1 min)");
            }
            catch (Exception ex)
            {
                HelperLog.Warn("Could not set restart-on-failure: " + ex.Message);
            }
        }

        private static CmdResult RunCaptured(string file, string args)
        {
            var psi = new ProcessStartInfo(file, args)
            {
                UseShellExecute = false,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                CreateNoWindow = true
            };
            var sb = new StringBuilder();
            using (var p = Process.Start(psi))
            {
                if (p == null)
                    throw new InvalidOperationException("Failed to start " + file);
                sb.Append(p.StandardOutput.ReadToEnd());
                sb.Append(p.StandardError.ReadToEnd());
                p.WaitForExit(20000);
                return new CmdResult { ExitCode = p.ExitCode, Output = sb.ToString().Trim() };
            }
        }

        private struct CmdResult
        {
            internal int ExitCode;
            internal string Output;
        }
    }

    internal static class NativeConsole
    {
        private const int SwHide = 0;

        [DllImport("kernel32.dll")]
        private static extern IntPtr GetConsoleWindow();

        [DllImport("user32.dll")]
        private static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);

        internal static void Hide()
        {
            try
            {
                var hwnd = GetConsoleWindow();
                if (hwnd != IntPtr.Zero)
                    ShowWindow(hwnd, SwHide);
            }
            catch
            {
            }
        }
    }
}
