using System;
using System.IO;
using System.IO.Pipes;
using System.Security.AccessControl;
using System.Security.Principal;
using System.Text;
using System.Threading;

namespace PulseCoreHelper
{
    internal sealed class NamedPipeHost : IDisposable
    {
        private readonly HardwareSampler _sampler;
        private readonly PipeSecurity _security;
        private readonly CancellationTokenSource _cts = new CancellationTokenSource();

        internal NamedPipeHost(HardwareSampler sampler, string userSid)
        {
            _sampler = sampler;
            _security = BuildSecurity(userSid);
        }

        internal void Run()
        {
            Console.CancelKeyPress += OnCancel;
            HelperLog.Info("Named pipe server listening on " + HelperPaths.PipeName);

            while (!_cts.IsCancellationRequested)
            {
                NamedPipeServerStream? pipe = null;
                try
                {
                    pipe = CreatePipe();
                    WaitForConnection(pipe);
                    if (_cts.IsCancellationRequested)
                        break;
                    if (pipe.IsConnected)
                        HandleClient(pipe);
                }
                catch (OperationCanceledException)
                {
                    break;
                }
                catch (Exception ex)
                {
                    if (!_cts.IsCancellationRequested)
                        HelperLog.Error("Pipe accept/handle failed: " + ex);
                    Thread.Sleep(200);
                }
                finally
                {
                    if (pipe != null)
                    {
                        try
                        {
                            if (pipe.IsConnected)
                                pipe.Disconnect();
                        }
                        catch
                        {
                        }

                        pipe.Dispose();
                    }
                }
            }
        }

        internal void Stop()
        {
            _cts.Cancel();
        }

        private void OnCancel(object sender, ConsoleCancelEventArgs e)
        {
            e.Cancel = true;
            _cts.Cancel();
        }

        private NamedPipeServerStream CreatePipe()
        {
            return new NamedPipeServerStream(
                HelperPaths.PipeName,
                PipeDirection.InOut,
                NamedPipeServerStream.MaxAllowedServerInstances,
                PipeTransmissionMode.Byte,
                PipeOptions.Asynchronous,
                16 * 1024,
                16 * 1024,
                _security);
        }

        private void WaitForConnection(NamedPipeServerStream pipe)
        {
            var async = pipe.BeginWaitForConnection(null, null);
            while (!async.IsCompleted)
            {
                if (_cts.IsCancellationRequested)
                {
                    try
                    {
                        pipe.Dispose();
                    }
                    catch
                    {
                    }

                    throw new OperationCanceledException();
                }

                async.AsyncWaitHandle.WaitOne(250);
            }

            pipe.EndWaitForConnection(async);
        }

        private void HandleClient(NamedPipeServerStream pipe)
        {
            var utf8 = new UTF8Encoding(false);
            using (var reader = new StreamReader(pipe, utf8, false, 4096, true))
            using (var writer = new StreamWriter(pipe, utf8, 4096, true) { AutoFlush = true, NewLine = "\n" })
            {
                while (pipe.IsConnected && !_cts.IsCancellationRequested)
                {
                    string? line;
                    try
                    {
                        line = reader.ReadLine();
                    }
                    catch (IOException)
                    {
                        break;
                    }

                    if (line == null)
                        break;
                    if (line.Length == 0)
                        continue;

                    string response;
                    try
                    {
                        response = Dispatch(line);
                    }
                    catch (Exception ex)
                    {
                        HelperLog.Error("Request failed: " + ex);
                        response = JsonUtil.ToLine(new HelperResponse
                        {
                            V = 1,
                            Ok = false,
                            Error = "unsupported"
                        });
                    }

                    try
                    {
                        writer.WriteLine(response);
                        writer.Flush();
                    }
                    catch (IOException)
                    {
                        break;
                    }
                }
            }
        }

        private string Dispatch(string line)
        {
            var req = JsonUtil.ParseRequest(line);
            var opRaw = req != null ? req.Op : null;
            if (req == null || req.V != 1 || string.IsNullOrWhiteSpace(opRaw))
                return Unsupported();

            var op = opRaw!.Trim().ToLowerInvariant();
            if (op == "ping")
            {
                return JsonUtil.ToLine(new HelperResponse { V = 1, Ok = true, Pong = true });
            }

            if (op == "get")
            {
                var sample = _sampler.Sample();
                return JsonUtil.ToLine(sample);
            }

            return Unsupported();
        }

        private static string Unsupported()
        {
            return JsonUtil.ToLine(new HelperResponse
            {
                V = 1,
                Ok = false,
                Error = "unsupported"
            });
        }

        /// <summary>
        /// DACL: installing user SID (needed for split-token medium IL) + Administrators + SYSTEM.
        /// No Everyone ACE. Avoid SACL/ML SDDL here - it can make Create() fail on .NET Framework.
        /// </summary>
        internal static PipeSecurity BuildSecurity(string userSid)
        {
            var ps = new PipeSecurity();
            ps.SetAccessRuleProtection(isProtected: true, preserveInheritance: false);

            var admins = new SecurityIdentifier(WellKnownSidType.BuiltinAdministratorsSid, null);
            var system = new SecurityIdentifier(WellKnownSidType.LocalSystemSid, null);
            ps.AddAccessRule(new PipeAccessRule(admins, PipeAccessRights.FullControl, AccessControlType.Allow));
            ps.AddAccessRule(new PipeAccessRule(system, PipeAccessRights.FullControl, AccessControlType.Allow));

            SecurityIdentifier? user = null;
            if (!string.IsNullOrWhiteSpace(userSid))
            {
                try { user = new SecurityIdentifier(userSid); }
                catch (Exception ex) { HelperLog.Warn("Invalid userSid: " + ex.Message); }
            }
            if (user == null)
            {
                using (var id = WindowsIdentity.GetCurrent())
                    user = id.User;
            }
            if (user != null)
            {
                // Explicit user SID - Administrators allow alone is deny-only for medium-IL token.
                ps.AddAccessRule(new PipeAccessRule(
                    user,
                    PipeAccessRights.ReadWrite | PipeAccessRights.CreateNewInstance | PipeAccessRights.Synchronize,
                    AccessControlType.Allow));
                HelperLog.Info("Pipe ACL userSid=" + user.Value);
            }
            return ps;
        }

        public void Dispose()
        {
            Console.CancelKeyPress -= OnCancel;
            _cts.Cancel();
            _cts.Dispose();
        }
    }
}
