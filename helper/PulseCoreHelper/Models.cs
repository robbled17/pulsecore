using System.Collections.Generic;
using Newtonsoft.Json;
using Newtonsoft.Json.Serialization;

namespace PulseCoreHelper
{
    public sealed class HelperRequest
    {
        [JsonProperty("v")]
        public int? V { get; set; }

        [JsonProperty("op")]
        public string? Op { get; set; }
    }

    public sealed class HelperResponse
    {
        [JsonProperty("v")]
        public int V { get; set; } = 1;

        [JsonProperty("ok")]
        public bool Ok { get; set; }

        [JsonProperty("pong", NullValueHandling = NullValueHandling.Ignore)]
        public bool? Pong { get; set; }

        [JsonProperty("error", NullValueHandling = NullValueHandling.Ignore)]
        public string? Error { get; set; }

        [JsonProperty("driver", NullValueHandling = NullValueHandling.Ignore)]
        public DriverInfo? Driver { get; set; }

        [JsonProperty("cpu", NullValueHandling = NullValueHandling.Ignore)]
        public CpuReading? Cpu { get; set; }

        [JsonProperty("disks", NullValueHandling = NullValueHandling.Ignore)]
        public List<DiskReading>? Disks { get; set; }

        [JsonProperty("ts", NullValueHandling = NullValueHandling.Ignore)]
        public string? Ts { get; set; }
    }

    public sealed class DriverInfo
    {
        [JsonProperty("pawnioRunning")]
        public bool PawnioRunning { get; set; }

        [JsonProperty("pawnioPresent")]
        public bool PawnioPresent { get; set; }

        [JsonProperty("blocked")]
        public bool Blocked { get; set; }

        [JsonProperty("blockedReason")]
        public string BlockedReason { get; set; } = "";

        [JsonProperty("backend")]
        public string Backend { get; set; } = "none";
    }

    public sealed class CpuReading
    {
        [JsonProperty("name")]
        public string Name { get; set; } = "";

        [JsonProperty("tctlC")]
        public double? TctlC { get; set; }

        [JsonProperty("tdieC")]
        public double? TdieC { get; set; }

        [JsonProperty("ccdTempsC")]
        public List<double> CcdTempsC { get; set; } = new List<double>();

        [JsonProperty("loadPercent")]
        public double? LoadPercent { get; set; }

        [JsonProperty("clockMHz")]
        public double? ClockMHz { get; set; }

        [JsonProperty("powerW")]
        public double? PowerW { get; set; }

        [JsonProperty("ok")]
        public bool Ok { get; set; }
    }

    public sealed class DiskReading
    {
        [JsonProperty("letter")]
        public string Letter { get; set; } = "";

        [JsonProperty("model")]
        public string Model { get; set; } = "";

        [JsonProperty("serial")]
        public string Serial { get; set; } = "";

        [JsonProperty("interface")]
        public string Interface { get; set; } = "";

        /// <summary>ssd|hdd|nvme|unknown - never guess SSD.</summary>
        [JsonProperty("mediaKind")]
        public string MediaKind { get; set; } = "unknown";

        [JsonProperty("isSsd")]
        public bool? IsSsd { get; set; }

        [JsonProperty("isNvme")]
        public bool IsNvme { get; set; }

        [JsonProperty("rotationRpm")]
        public int? RotationRpm { get; set; }

        [JsonProperty("identifyWord217")]
        public int? IdentifyWord217 { get; set; }

        [JsonProperty("tempC")]
        public double? TempC { get; set; }

        [JsonProperty("tempOk")]
        public bool TempOk { get; set; }

        [JsonProperty("healthPercent")]
        public double? HealthPercent { get; set; }

        [JsonProperty("healthOk")]
        public bool HealthOk { get; set; }

        [JsonProperty("powerOnHours")]
        public long? PowerOnHours { get; set; }

        [JsonProperty("totalWritesBytes")]
        public long? TotalWritesBytes { get; set; }

        [JsonProperty("status")]
        public string Status { get; set; } = "unknown";

        [JsonProperty("attrs")]
        public List<SmartAttr> Attrs { get; set; } = new List<SmartAttr>();
    }

    public sealed class SmartAttr
    {
        [JsonProperty("id")]
        public int Id { get; set; }

        [JsonProperty("name")]
        public string Name { get; set; } = "";

        [JsonProperty("current")]
        public int Current { get; set; }

        [JsonProperty("worst")]
        public int Worst { get; set; }

        [JsonProperty("threshold")]
        public int Threshold { get; set; }

        [JsonProperty("raw")]
        public string Raw { get; set; } = "";
    }

    public sealed class NamedSensor
    {
        public NamedSensor()
        {
        }

        public NamedSensor(string name, string sensorType, double? value)
        {
            Name = name ?? "";
            SensorType = sensorType ?? "";
            Value = value;
        }

        public string Name { get; set; } = "";
        public string SensorType { get; set; } = "";
        public double? Value { get; set; }
    }

    public sealed class HelperState
    {
        [JsonProperty("userSid")]
        public string UserSid { get; set; } = "";

        [JsonProperty("installedByPulseCore")]
        public bool InstalledByPulseCore { get; set; }

        [JsonProperty("enabled")]
        public bool Enabled { get; set; }

        [JsonProperty("pawnioInstalledByUs")]
        public bool PawnioInstalledByUs { get; set; }

        [JsonProperty("taskName")]
        public string TaskName { get; set; } = "PulseCoreHelper";

        [JsonProperty("installedAt")]
        public string InstalledAt { get; set; } = "";

        [JsonProperty("helperPath")]
        public string HelperPath { get; set; } = "";

        [JsonProperty("helperVersion")]
        public string HelperVersion { get; set; } = "";

        [JsonProperty("protocolVersion")]
        public int ProtocolVersion { get; set; }

        [JsonProperty("helperSha256")]
        public string HelperSha256 { get; set; } = "";
    }

    internal static class JsonUtil
    {
        internal static readonly JsonSerializerSettings Settings = new JsonSerializerSettings
        {
            ContractResolver = new DefaultContractResolver(),
            NullValueHandling = NullValueHandling.Include,
            MissingMemberHandling = MissingMemberHandling.Ignore,
            Formatting = Formatting.None
        };

        internal static string ToLine(object value)
        {
            return JsonConvert.SerializeObject(value, Settings);
        }

        internal static HelperRequest? ParseRequest(string line)
        {
            if (string.IsNullOrWhiteSpace(line))
                return null;
            try
            {
                return JsonConvert.DeserializeObject<HelperRequest>(line, Settings);
            }
            catch
            {
                return null;
            }
        }
    }
}
