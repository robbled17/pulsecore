export namespace main {
	
	export class AppUsageEntry {
	    name: string;
	    friendlyName: string;
	    trafficMB: number;
	    energymWh: number;
	    activeHours: number;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new AppUsageEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.friendlyName = source["friendlyName"];
	        this.trafficMB = source["trafficMB"];
	        this.energymWh = source["energymWh"];
	        this.activeHours = source["activeHours"];
	        this.detail = source["detail"];
	    }
	}
	export class BatteryMetrics {
	    percent: number;
	    pluggedIn: boolean;
	    status: string;
	    statusText: string;
	    powerDrawWatts: number;
	    available: boolean;
	    wearPercent: number;
	    wearLabel: string;
	    capacitymWh: number;
	    designCapacitymWh: number;
	    deviceName: string;
	
	    static createFrom(source: any = {}) {
	        return new BatteryMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.percent = source["percent"];
	        this.pluggedIn = source["pluggedIn"];
	        this.status = source["status"];
	        this.statusText = source["statusText"];
	        this.powerDrawWatts = source["powerDrawWatts"];
	        this.available = source["available"];
	        this.wearPercent = source["wearPercent"];
	        this.wearLabel = source["wearLabel"];
	        this.capacitymWh = source["capacitymWh"];
	        this.designCapacitymWh = source["designCapacitymWh"];
	        this.deviceName = source["deviceName"];
	    }
	}
	export class CPUDeepInfo {
	    name: string;
	    physicalCores: number;
	    logicalCores: number;
	    frequencyMHz: number;
	    perCore: number[];
	    percent: number;
	
	    static createFrom(source: any = {}) {
	        return new CPUDeepInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.physicalCores = source["physicalCores"];
	        this.logicalCores = source["logicalCores"];
	        this.frequencyMHz = source["frequencyMHz"];
	        this.perCore = source["perCore"];
	        this.percent = source["percent"];
	    }
	}
	export class DiskHealthReport {
	    driveLetter: string;
	    model: string;
	    serialNumber: string;
	    mediaType: string;
	    interfaceType: string;
	    healthStatus: string;
	    operationalStatus: string;
	    verdict: string;
	    verdictBadge: string;
	    healthPercent: number;
	    temperatureC: number;
	    wearRemaining: number;
	    powerOnHours: number;
	    tbwGB: number;
	    hasCriticalErrors: boolean;
	    readErrors: number;
	    writeErrors: number;
	    errorMessage: string;
	    ok: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DiskHealthReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.driveLetter = source["driveLetter"];
	        this.model = source["model"];
	        this.serialNumber = source["serialNumber"];
	        this.mediaType = source["mediaType"];
	        this.interfaceType = source["interfaceType"];
	        this.healthStatus = source["healthStatus"];
	        this.operationalStatus = source["operationalStatus"];
	        this.verdict = source["verdict"];
	        this.verdictBadge = source["verdictBadge"];
	        this.healthPercent = source["healthPercent"];
	        this.temperatureC = source["temperatureC"];
	        this.wearRemaining = source["wearRemaining"];
	        this.powerOnHours = source["powerOnHours"];
	        this.tbwGB = source["tbwGB"];
	        this.hasCriticalErrors = source["hasCriticalErrors"];
	        this.readErrors = source["readErrors"];
	        this.writeErrors = source["writeErrors"];
	        this.errorMessage = source["errorMessage"];
	        this.ok = source["ok"];
	    }
	}
	export class DiskMetrics {
	    mount: string;
	    usedGB: number;
	    totalGB: number;
	    percent: number;
	    readMBps: number;
	    writeMBps: number;
	    freeGB: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mount = source["mount"];
	        this.usedGB = source["usedGB"];
	        this.totalGB = source["totalGB"];
	        this.percent = source["percent"];
	        this.readMBps = source["readMBps"];
	        this.writeMBps = source["writeMBps"];
	        this.freeGB = source["freeGB"];
	    }
	}
	export class FPSPrediction {
	    gameId: string;
	    gameName: string;
	    posterUrl: string;
	    headerUrl: string;
	    genre: string;
	    badge: string;
	    engine: string;
	    year: number;
	    cpuScore: number;
	    gpuScore: number;
	    dualChannel: boolean;
	    isIntegrated: boolean;
	    vramGB: number;
	    ramGB: number;
	    cpuCores: number;
	    lowAvg: number;
	    medAvg: number;
	    highAvg: number;
	    ultraAvg: number;
	    lowRange: string;
	    medRange: string;
	    highRange: string;
	    ultraRange: string;
	    recommend: string;
	    note: string;
	    warnings: string[];
	    category: string;
	    verdict: string;
	    verdictLevel: string;
	
	    static createFrom(source: any = {}) {
	        return new FPSPrediction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameId = source["gameId"];
	        this.gameName = source["gameName"];
	        this.posterUrl = source["posterUrl"];
	        this.headerUrl = source["headerUrl"];
	        this.genre = source["genre"];
	        this.badge = source["badge"];
	        this.engine = source["engine"];
	        this.year = source["year"];
	        this.cpuScore = source["cpuScore"];
	        this.gpuScore = source["gpuScore"];
	        this.dualChannel = source["dualChannel"];
	        this.isIntegrated = source["isIntegrated"];
	        this.vramGB = source["vramGB"];
	        this.ramGB = source["ramGB"];
	        this.cpuCores = source["cpuCores"];
	        this.lowAvg = source["lowAvg"];
	        this.medAvg = source["medAvg"];
	        this.highAvg = source["highAvg"];
	        this.ultraAvg = source["ultraAvg"];
	        this.lowRange = source["lowRange"];
	        this.medRange = source["medRange"];
	        this.highRange = source["highRange"];
	        this.ultraRange = source["ultraRange"];
	        this.recommend = source["recommend"];
	        this.note = source["note"];
	        this.warnings = source["warnings"];
	        this.category = source["category"];
	        this.verdict = source["verdict"];
	        this.verdictLevel = source["verdictLevel"];
	    }
	}
	export class GPUDeepInfo {
	    name: string;
	    driverVersion: string;
	    dedicatedMB: number;
	    sharedMB: number;
	    usedMB: number;
	    percent: number;
	    isIntegrated: boolean;
	    vramGB: number;
	
	    static createFrom(source: any = {}) {
	        return new GPUDeepInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.driverVersion = source["driverVersion"];
	        this.dedicatedMB = source["dedicatedMB"];
	        this.sharedMB = source["sharedMB"];
	        this.usedMB = source["usedMB"];
	        this.percent = source["percent"];
	        this.isIntegrated = source["isIntegrated"];
	        this.vramGB = source["vramGB"];
	    }
	}
	export class GameInfo {
	    id: string;
	    name: string;
	    genre: string;
	    weight: number;
	    steamAppId: number;
	    posterUrl: string;
	    headerUrl?: string;
	    category: string;
	    year?: number;
	    badge?: string;
	    engine?: string;
	    synthetic?: boolean;
	    fromSteam?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GameInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.genre = source["genre"];
	        this.weight = source["weight"];
	        this.steamAppId = source["steamAppId"];
	        this.posterUrl = source["posterUrl"];
	        this.headerUrl = source["headerUrl"];
	        this.category = source["category"];
	        this.year = source["year"];
	        this.badge = source["badge"];
	        this.engine = source["engine"];
	        this.synthetic = source["synthetic"];
	        this.fromSteam = source["fromSteam"];
	    }
	}
	export class PowerDeepInfo {
	    hasBattery: boolean;
	    wearPercent: number;
	    wearLabel: string;
	    pluggedIn: boolean;
	    capacitymWh: number;
	    designCapacitymWh: number;
	    percent: number;
	    statusText: string;
	    powerDrawWatts: number;
	    deviceName: string;
	
	    static createFrom(source: any = {}) {
	        return new PowerDeepInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasBattery = source["hasBattery"];
	        this.wearPercent = source["wearPercent"];
	        this.wearLabel = source["wearLabel"];
	        this.pluggedIn = source["pluggedIn"];
	        this.capacitymWh = source["capacitymWh"];
	        this.designCapacitymWh = source["designCapacitymWh"];
	        this.percent = source["percent"];
	        this.statusText = source["statusText"];
	        this.powerDrawWatts = source["powerDrawWatts"];
	        this.deviceName = source["deviceName"];
	    }
	}
	export class RAMDeepInfo {
	    totalMB: number;
	    usedMB: number;
	    freeMB: number;
	    percent: number;
	    type: string;
	    slotsUsed: number;
	    slotsTotal: number;
	    speedMHz: number;
	    isSingleChannel: boolean;
	    channelStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new RAMDeepInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalMB = source["totalMB"];
	        this.usedMB = source["usedMB"];
	        this.freeMB = source["freeMB"];
	        this.percent = source["percent"];
	        this.type = source["type"];
	        this.slotsUsed = source["slotsUsed"];
	        this.slotsTotal = source["slotsTotal"];
	        this.speedMHz = source["speedMHz"];
	        this.isSingleChannel = source["isSingleChannel"];
	        this.channelStatus = source["channelStatus"];
	    }
	}
	export class HardwareInfo {
	    cpu: CPUDeepInfo;
	    gpu: GPUDeepInfo;
	    ram: RAMDeepInfo;
	    power: PowerDeepInfo;
	
	    static createFrom(source: any = {}) {
	        return new HardwareInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpu = this.convertValues(source["cpu"], CPUDeepInfo);
	        this.gpu = this.convertValues(source["gpu"], GPUDeepInfo);
	        this.ram = this.convertValues(source["ram"], RAMDeepInfo);
	        this.power = this.convertValues(source["power"], PowerDeepInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LagDiagnosis {
	    gatewayHost: string;
	    gatewayMs: number;
	    gatewayBadge: string;
	    gatewayBadgeOK: boolean;
	    publicHost: string;
	    publicMs: number;
	    publicBadge: string;
	    publicBadgeOK: boolean;
	    localJitterMs: number;
	    jitterBadge: string;
	    jitterBadgeOK: boolean;
	    verdict: string;
	    verdictKey: string;
	    level: string;
	    ok: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LagDiagnosis(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gatewayHost = source["gatewayHost"];
	        this.gatewayMs = source["gatewayMs"];
	        this.gatewayBadge = source["gatewayBadge"];
	        this.gatewayBadgeOK = source["gatewayBadgeOK"];
	        this.publicHost = source["publicHost"];
	        this.publicMs = source["publicMs"];
	        this.publicBadge = source["publicBadge"];
	        this.publicBadgeOK = source["publicBadgeOK"];
	        this.localJitterMs = source["localJitterMs"];
	        this.jitterBadge = source["jitterBadge"];
	        this.jitterBadgeOK = source["jitterBadgeOK"];
	        this.verdict = source["verdict"];
	        this.verdictKey = source["verdictKey"];
	        this.level = source["level"];
	        this.ok = source["ok"];
	    }
	}
	export class LagEvent {
	    timestamp: string;
	    coreID: number;
	    coreLoad: number;
	    overallLoad: number;
	    culpritProcess: string;
	    culpritFriendly: string;
	    culpritPID: number;
	    explanation: string;
	    icon: string;
	
	    static createFrom(source: any = {}) {
	        return new LagEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.coreID = source["coreID"];
	        this.coreLoad = source["coreLoad"];
	        this.overallLoad = source["overallLoad"];
	        this.culpritProcess = source["culpritProcess"];
	        this.culpritFriendly = source["culpritFriendly"];
	        this.culpritPID = source["culpritPID"];
	        this.explanation = source["explanation"];
	        this.icon = source["icon"];
	    }
	}
	export class MonthlyAppReport {
	    ok: boolean;
	    period: string;
	    topApps: AppUsageEntry[];
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new MonthlyAppReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.period = source["period"];
	        this.topApps = this.convertValues(source["topApps"], AppUsageEntry);
	        this.note = source["note"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PeripheralItem {
	    name: string;
	    deviceID: string;
	    service: string;
	    class: string;
	    kind: string;
	    group: string;
	    icon: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new PeripheralItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.deviceID = source["deviceID"];
	        this.service = source["service"];
	        this.class = source["class"];
	        this.kind = source["kind"];
	        this.group = source["group"];
	        this.icon = source["icon"];
	        this.status = source["status"];
	    }
	}
	
	
	export class SleepReport {
	    ok: boolean;
	    sleepOK: boolean;
	    summary: string;
	    blockers: string[];
	    rawHint: string;
	
	    static createFrom(source: any = {}) {
	        return new SleepReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.sleepOK = source["sleepOK"];
	        this.summary = source["summary"];
	        this.blockers = source["blockers"];
	        this.rawHint = source["rawHint"];
	    }
	}
	export class StutterEvent {
	    timestamp: string;
	    jitterMs: number;
	    cause: string;
	    severity: string;
	    culprit: string;
	    diskBusy: boolean;
	    freeRAMMB: number;
	    diskReadMBps: number;
	    diskWriteMBps: number;
	
	    static createFrom(source: any = {}) {
	        return new StutterEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.jitterMs = source["jitterMs"];
	        this.cause = source["cause"];
	        this.severity = source["severity"];
	        this.culprit = source["culprit"];
	        this.diskBusy = source["diskBusy"];
	        this.freeRAMMB = source["freeRAMMB"];
	        this.diskReadMBps = source["diskReadMBps"];
	        this.diskWriteMBps = source["diskWriteMBps"];
	    }
	}
	export class VampireProcess {
	    pid: number;
	    pids: number[];
	    count: number;
	    name: string;
	    friendlyName: string;
	    cpuPercent: number;
	    diskMBps: number;
	    score: number;
	    ecoMode: boolean;
	    frozen: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VampireProcess(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.pids = source["pids"];
	        this.count = source["count"];
	        this.name = source["name"];
	        this.friendlyName = source["friendlyName"];
	        this.cpuPercent = source["cpuPercent"];
	        this.diskMBps = source["diskMBps"];
	        this.score = source["score"];
	        this.ecoMode = source["ecoMode"];
	        this.frozen = source["frozen"];
	    }
	}
	export class SystemMetrics {
	    timestamp: string;
	    cpuName: string;
	    cpuPercent: number;
	    cpuPerCore: number[];
	    gpuName: string;
	    gpuPercent: number;
	    vramUsedMB: number;
	    vramTotalMB: number;
	    vramSharedMB: number;
	    vramDedicatedMB: number;
	    ramPercent: number;
	    ramUsedMB: number;
	    ramTotalMB: number;
	    swapPercent: number;
	    swapUsedMB: number;
	    swapTotalMB: number;
	    disks: DiskMetrics[];
	    battery: BatteryMetrics;
	    hardware: HardwareInfo;
	    temperatureC: number;
	    powerMode: string;
	    pollIntervalMS: number;
	    miniHUD: boolean;
	    language: string;
	    lagEvents: LagEvent[];
	    stutterEvents: StutterEvent[];
	    vampires: VampireProcess[];
	
	    static createFrom(source: any = {}) {
	        return new SystemMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.cpuName = source["cpuName"];
	        this.cpuPercent = source["cpuPercent"];
	        this.cpuPerCore = source["cpuPerCore"];
	        this.gpuName = source["gpuName"];
	        this.gpuPercent = source["gpuPercent"];
	        this.vramUsedMB = source["vramUsedMB"];
	        this.vramTotalMB = source["vramTotalMB"];
	        this.vramSharedMB = source["vramSharedMB"];
	        this.vramDedicatedMB = source["vramDedicatedMB"];
	        this.ramPercent = source["ramPercent"];
	        this.ramUsedMB = source["ramUsedMB"];
	        this.ramTotalMB = source["ramTotalMB"];
	        this.swapPercent = source["swapPercent"];
	        this.swapUsedMB = source["swapUsedMB"];
	        this.swapTotalMB = source["swapTotalMB"];
	        this.disks = this.convertValues(source["disks"], DiskMetrics);
	        this.battery = this.convertValues(source["battery"], BatteryMetrics);
	        this.hardware = this.convertValues(source["hardware"], HardwareInfo);
	        this.temperatureC = source["temperatureC"];
	        this.powerMode = source["powerMode"];
	        this.pollIntervalMS = source["pollIntervalMS"];
	        this.miniHUD = source["miniHUD"];
	        this.language = source["language"];
	        this.lagEvents = this.convertValues(source["lagEvents"], LagEvent);
	        this.stutterEvents = this.convertValues(source["stutterEvents"], StutterEvent);
	        this.vampires = this.convertValues(source["vampires"], VampireProcess);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SystemPowerProfile {
	    gpuName: string;
	    isIntegrated: boolean;
	    vramGB: number;
	    gpuPowerScore: number;
	    cpuName: string;
	    cpuCores: number;
	    cpuThreads: number;
	    cpuFreqMHz: number;
	    cpuPowerScore: number;
	    ramGB: number;
	    isDualChannel: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SystemPowerProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gpuName = source["gpuName"];
	        this.isIntegrated = source["isIntegrated"];
	        this.vramGB = source["vramGB"];
	        this.gpuPowerScore = source["gpuPowerScore"];
	        this.cpuName = source["cpuName"];
	        this.cpuCores = source["cpuCores"];
	        this.cpuThreads = source["cpuThreads"];
	        this.cpuFreqMHz = source["cpuFreqMHz"];
	        this.cpuPowerScore = source["cpuPowerScore"];
	        this.ramGB = source["ramGB"];
	        this.isDualChannel = source["isDualChannel"];
	    }
	}

}

