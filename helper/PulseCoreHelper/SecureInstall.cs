using System;
using System.Diagnostics;
using System.IO;
using System.Security.AccessControl;
using System.Security.Cryptography;
using System.Security.Principal;
using System.Text;

namespace PulseCoreHelper
{
    /// <summary>
    /// Copies helper into an admin-only directory under Program Files and hardens ACLs.
    /// </summary>
    internal static class SecureInstall
    {
        internal const string HelperVersion = "1.0.0";
        internal const int ProtocolVersion = 1;

        internal static string ProgramFilesHelperDir
        {
            get
            {
                var pf = Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles);
                if (string.IsNullOrEmpty(pf))
                    pf = @"C:\Program Files";
                return Path.Combine(pf, "PulseCore", "helper");
            }
        }

        internal static string ProgramFilesHelperExe =>
            Path.Combine(ProgramFilesHelperDir, "PulseCoreHelper.exe");

        internal static string DeployFromSource(string srcDir)
        {
            if (string.IsNullOrWhiteSpace(srcDir) || !Directory.Exists(srcDir))
                throw new DirectoryNotFoundException("Helper source not found: " + srcDir);

            var srcExe = Path.Combine(srcDir, "PulseCoreHelper.exe");
            if (!File.Exists(srcExe))
                throw new FileNotFoundException("PulseCoreHelper.exe missing in source", srcExe);

            var dst = ProgramFilesHelperDir;
            Directory.CreateDirectory(dst);

            foreach (var file in Directory.GetFiles(srcDir, "*", SearchOption.TopDirectoryOnly))
            {
                var name = Path.GetFileName(file);
                if (string.Equals(name, "PulseCoreHelper.pdb", StringComparison.OrdinalIgnoreCase))
                    continue;
                File.Copy(file, Path.Combine(dst, name), overwrite: true);
            }

            HardenDirectoryAcl(dst);
            var hash = Sha256File(ProgramFilesHelperExe);
            File.WriteAllText(Path.Combine(dst, "helper.sha256"), hash + Environment.NewLine, Encoding.ASCII);
            File.WriteAllText(
                Path.Combine(dst, "helper.version"),
                HelperVersion + "|" + ProtocolVersion + Environment.NewLine,
                Encoding.ASCII);
            HelperLog.Info("Deployed helper to " + dst + " sha256=" + hash);
            return ProgramFilesHelperExe;
        }

        internal static void HardenDirectoryAcl(string dir)
        {
            var di = new DirectoryInfo(dir);
            var ds = di.GetAccessControl();
            ds.SetAccessRuleProtection(isProtected: true, preserveInheritance: false);

            var admins = new SecurityIdentifier(WellKnownSidType.BuiltinAdministratorsSid, null);
            var system = new SecurityIdentifier(WellKnownSidType.LocalSystemSid, null);
            var users = new SecurityIdentifier(WellKnownSidType.BuiltinUsersSid, null);

            ds.PurgeAccessRules(admins);
            ds.PurgeAccessRules(system);
            ds.PurgeAccessRules(users);

            ds.AddAccessRule(new FileSystemAccessRule(
                admins, FileSystemRights.FullControl,
                InheritanceFlags.ContainerInherit | InheritanceFlags.ObjectInherit,
                PropagationFlags.None, AccessControlType.Allow));
            ds.AddAccessRule(new FileSystemAccessRule(
                system, FileSystemRights.FullControl,
                InheritanceFlags.ContainerInherit | InheritanceFlags.ObjectInherit,
                PropagationFlags.None, AccessControlType.Allow));
            // Authenticated users: read+execute only (no write).
            ds.AddAccessRule(new FileSystemAccessRule(
                users, FileSystemRights.ReadAndExecute,
                InheritanceFlags.ContainerInherit | InheritanceFlags.ObjectInherit,
                PropagationFlags.None, AccessControlType.Allow));

            di.SetAccessControl(ds);
        }

        internal static string Sha256File(string path)
        {
            using (var fs = File.OpenRead(path))
            using (var sha = SHA256.Create())
            {
                var hash = sha.ComputeHash(fs);
                var sb = new StringBuilder(hash.Length * 2);
                foreach (var b in hash)
                    sb.Append(b.ToString("X2"));
                return sb.ToString();
            }
        }

        internal static bool VerifyInstalledIntegrity(out string reason)
        {
            reason = "";
            var exe = ProgramFilesHelperExe;
            if (!File.Exists(exe))
            {
                reason = "helper exe missing in Program Files";
                return false;
            }

            var hashFile = Path.Combine(ProgramFilesHelperDir, "helper.sha256");
            if (!File.Exists(hashFile))
            {
                reason = "helper.sha256 missing";
                return false;
            }

            var want = File.ReadAllText(hashFile).Trim();
            var got = Sha256File(exe);
            if (!string.Equals(want, got, StringComparison.OrdinalIgnoreCase))
            {
                reason = "helper SHA256 mismatch";
                return false;
            }

            return true;
        }

        internal static void RemoveProgramFilesHelper()
        {
            try
            {
                if (Directory.Exists(ProgramFilesHelperDir))
                    Directory.Delete(ProgramFilesHelperDir, recursive: true);
            }
            catch (Exception ex)
            {
                HelperLog.Warn("Failed removing Program Files helper: " + ex.Message);
            }

            try
            {
                var root = Path.GetDirectoryName(ProgramFilesHelperDir);
                if (!string.IsNullOrEmpty(root) && Directory.Exists(root) &&
                    Directory.GetFileSystemEntries(root).Length == 0)
                    Directory.Delete(root);
            }
            catch
            {
            }
        }
    }
}
