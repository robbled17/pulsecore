export namespace loc {
	
	export class Msg {
	    code: string;
	    params?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Msg(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.params = source["params"];
	    }
	}

}

export namespace main {
	
	export class ActiveGameStatus {
	    active: boolean;
	    name: string;
	    exe: string;
	    durationMin: number;
	    cpuPercent: number;
	    gpuPercent: number;
	    ramPercent: number;
	    tempCPU: number;
	    tempGPU: number;
	    freezes: number;
	    fpsNote: string;
	
	    static createFrom(source: any = {}) {
	        return new ActiveGameStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.name = source["name"];
	        this.exe = source["exe"];
	        this.durationMin = source["durationMin"];
	        this.cpuPercent = source["cpuPercent"];
	        this.gpuPercent = source["gpuPercent"];
	        this.ramPercent = source["ramPercent"];
	        this.tempCPU = source["tempCPU"];
	        this.tempGPU = source["tempGPU"];
	        this.freezes = source["freezes"];
	        this.fpsNote = source["fpsNote"];
	    }
	}
	export class AdvancedModeState {
	    enabled: boolean;
	    helperRunning: boolean;
	    status: string;
	    phase: string;
	    statusRu: string;
	    statusEn: string;
	    driverBlocked: boolean;
	    driverBlockedReason: string;
	    pawnioPresent: boolean;
	    pawnioOwnedByUs: boolean;
	    explainRu: string;
	    explainEn: string;
	    lastError?: string;
	    errorCode?: string;
	    actionRu?: string;
	    actionEn?: string;
	    taskRunning: boolean;
	    processRunning: boolean;
	    processPid: number;
	    pipePresent: boolean;
	    protocolVersion: number;
	    helperVersion: string;
	    lastHelperReplyAt?: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedModeState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.helperRunning = source["helperRunning"];
	        this.status = source["status"];
	        this.phase = source["phase"];
	        this.statusRu = source["statusRu"];
	        this.statusEn = source["statusEn"];
	        this.driverBlocked = source["driverBlocked"];
	        this.driverBlockedReason = source["driverBlockedReason"];
	        this.pawnioPresent = source["pawnioPresent"];
	        this.pawnioOwnedByUs = source["pawnioOwnedByUs"];
	        this.explainRu = source["explainRu"];
	        this.explainEn = source["explainEn"];
	        this.lastError = source["lastError"];
	        this.errorCode = source["errorCode"];
	        this.actionRu = source["actionRu"];
	        this.actionEn = source["actionEn"];
	        this.taskRunning = source["taskRunning"];
	        this.processRunning = source["processRunning"];
	        this.processPid = source["processPid"];
	        this.pipePresent = source["pipePresent"];
	        this.protocolVersion = source["protocolVersion"];
	        this.helperVersion = source["helperVersion"];
	        this.lastHelperReplyAt = source["lastHelperReplyAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class AdvancedModeActionResult {
	    ok: boolean;
	    message: loc.Msg;
	    messageRu: string;
	    messageEn: string;
	    state: AdvancedModeState;
	    needConsentPawnIO?: boolean;
	    errorCode?: string;
	    phase?: string;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedModeActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = this.convertValues(source["message"], loc.Msg);
	        this.messageRu = source["messageRu"];
	        this.messageEn = source["messageEn"];
	        this.state = this.convertValues(source["state"], AdvancedModeState);
	        this.needConsentPawnIO = source["needConsentPawnIO"];
	        this.errorCode = source["errorCode"];
	        this.phase = source["phase"];
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
	
	export class AppLeftover {
	    path: string;
	    sizeBytes: number;
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppLeftover(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.sizeBytes = source["sizeBytes"];
	        this.exists = source["exists"];
	    }
	}
	export class AppUsageEntry {
	    name: string;
	    friendlyName: string;
	    trafficMB: number;
	    energymWh: number;
	    activeHours: number;
	    cpuSec: number;
	    diskMB: number;
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
	        this.cpuSec = source["cpuSec"];
	        this.diskMB = source["diskMB"];
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
	export class CleanRecord {
	    timestamp: time.Time;
	    bytes_before: number;
	    bytes_after: number;
	    bytes_freed: number;
	    deletedBytes: number;
	    total_bytes: number;
	    target_drive: string;
	    category: string;
	
	    static createFrom(source: any = {}) {
	        return new CleanRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], time.Time);
	        this.bytes_before = source["bytes_before"];
	        this.bytes_after = source["bytes_after"];
	        this.bytes_freed = source["bytes_freed"];
	        this.deletedBytes = source["deletedBytes"];
	        this.total_bytes = source["total_bytes"];
	        this.target_drive = source["target_drive"];
	        this.category = source["category"];
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
	export class CleanStats {
	    last_clean_freed: number;
	    last_clean_before: number;
	    last_clean_after: number;
	    last_used_before: number;
	    last_used_after: number;
	    last_total_bytes: number;
	    last_category: string;
	    last_drive: string;
	    today_freed: number;
	    month_freed: number;
	    all_time_freed: number;
	
	    static createFrom(source: any = {}) {
	        return new CleanStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.last_clean_freed = source["last_clean_freed"];
	        this.last_clean_before = source["last_clean_before"];
	        this.last_clean_after = source["last_clean_after"];
	        this.last_used_before = source["last_used_before"];
	        this.last_used_after = source["last_used_after"];
	        this.last_total_bytes = source["last_total_bytes"];
	        this.last_category = source["last_category"];
	        this.last_drive = source["last_drive"];
	        this.today_freed = source["today_freed"];
	        this.month_freed = source["month_freed"];
	        this.all_time_freed = source["all_time_freed"];
	    }
	}
	export class CleanupCat {
	    id: string;
	    sizeBytes: number;
	    needsAdmin: boolean;
	    irreversible: boolean;
	    available: boolean;
	    selected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CleanupCat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sizeBytes = source["sizeBytes"];
	        this.needsAdmin = source["needsAdmin"];
	        this.irreversible = source["irreversible"];
	        this.available = source["available"];
	        this.selected = source["selected"];
	    }
	}
	export class CleanupResult {
	    ok: boolean;
	    freedBytes: number;
	    actualCleanedBytes: number;
	    categories: string[];
	    usedBefore: number;
	    usedAfter: number;
	    freeBefore: number;
	    freeAfter: number;
	    totalBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new CleanupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.freedBytes = source["freedBytes"];
	        this.actualCleanedBytes = source["actualCleanedBytes"];
	        this.categories = source["categories"];
	        this.usedBefore = source["usedBefore"];
	        this.usedAfter = source["usedAfter"];
	        this.freeBefore = source["freeBefore"];
	        this.freeAfter = source["freeAfter"];
	        this.totalBytes = source["totalBytes"];
	    }
	}
	export class DayStats {
	    date: string;
	    cpuTempMax: number;
	    cpuTempSum: number;
	    cpuTempN: number;
	    gpuTempMax: number;
	    gpuTempSum: number;
	    gpuTempN: number;
	    incidents: number;
	    culprits: Record<string, number>;
	    gameMinutes: Record<string, number>;
	    highLoadMinutes: number;
	    ecoRAMFreedMB: number;
	    ecoOptimizations: number;
	    cpuTempAvg: number;
	    gpuTempAvg: number;
	
	    static createFrom(source: any = {}) {
	        return new DayStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.cpuTempMax = source["cpuTempMax"];
	        this.cpuTempSum = source["cpuTempSum"];
	        this.cpuTempN = source["cpuTempN"];
	        this.gpuTempMax = source["gpuTempMax"];
	        this.gpuTempSum = source["gpuTempSum"];
	        this.gpuTempN = source["gpuTempN"];
	        this.incidents = source["incidents"];
	        this.culprits = source["culprits"];
	        this.gameMinutes = source["gameMinutes"];
	        this.highLoadMinutes = source["highLoadMinutes"];
	        this.ecoRAMFreedMB = source["ecoRAMFreedMB"];
	        this.ecoOptimizations = source["ecoOptimizations"];
	        this.cpuTempAvg = source["cpuTempAvg"];
	        this.gpuTempAvg = source["gpuTempAvg"];
	    }
	}
	export class DeepCleanupItem {
	    id: string;
	    sizeBytes: number;
	    available: boolean;
	    needsAdmin: boolean;
	    irreversible: boolean;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeepCleanupItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sizeBytes = source["sizeBytes"];
	        this.available = source["available"];
	        this.needsAdmin = source["needsAdmin"];
	        this.irreversible = source["irreversible"];
	        this.enabled = source["enabled"];
	    }
	}
	export class DeepCleanupPreview {
	    items: DeepCleanupItem[];
	    hibernationOn: boolean;
	    hibernationBytes: number;
	    shadowBytes: number;
	    deliveryOptBytes: number;
	    systemDriveUsedPct: number;
	    systemDriveFreeGB: number;
	
	    static createFrom(source: any = {}) {
	        return new DeepCleanupPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], DeepCleanupItem);
	        this.hibernationOn = source["hibernationOn"];
	        this.hibernationBytes = source["hibernationBytes"];
	        this.shadowBytes = source["shadowBytes"];
	        this.deliveryOptBytes = source["deliveryOptBytes"];
	        this.systemDriveUsedPct = source["systemDriveUsedPct"];
	        this.systemDriveFreeGB = source["systemDriveFreeGB"];
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
	export class DeepCleanupResult {
	    ok: boolean;
	    freedBytes: number;
	    actualCleanedBytes: number;
	    actions: string[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DeepCleanupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.freedBytes = source["freedBytes"];
	        this.actualCleanedBytes = source["actualCleanedBytes"];
	        this.actions = source["actions"];
	        this.message = source["message"];
	    }
	}
	export class DiagnosticExport {
	    ok: boolean;
	    format: string;
	    filename: string;
	    content: string;
	    messageRu: string;
	    messageEn: string;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticExport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.format = source["format"];
	        this.filename = source["filename"];
	        this.content = source["content"];
	        this.messageRu = source["messageRu"];
	        this.messageEn = source["messageEn"];
	    }
	}
	export class ProcessSnap {
	    name: string;
	    pid: number;
	    pids?: number[];
	    cpuPercent: number;
	    ramBytes: number;
	    diskMBps: number;
	    diskReadMBps: number;
	    diskWriteMBps: number;
	
	    static createFrom(source: any = {}) {
	        return new ProcessSnap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.pid = source["pid"];
	        this.pids = source["pids"];
	        this.cpuPercent = source["cpuPercent"];
	        this.ramBytes = source["ramBytes"];
	        this.diskMBps = source["diskMBps"];
	        this.diskReadMBps = source["diskReadMBps"];
	        this.diskWriteMBps = source["diskWriteMBps"];
	    }
	}
	export class EcoTarget {
	    name: string;
	    pid: number;
	    cpuPercent: number;
	
	    static createFrom(source: any = {}) {
	        return new EcoTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.pid = source["pid"];
	        this.cpuPercent = source["cpuPercent"];
	    }
	}
	export class IncidentCard {
	    id: string;
	    time: string;
	    durationMs: number;
	    type: string;
	    verdictText: string;
	    culprit: string;
	    culpritPID: number;
	    confidence: number;
	    evidence: string[];
	    evidenceRaw: string[];
	    ecoTargets: EcoTarget[];
	    canEco: boolean;
	    systemProc: boolean;
	    synthetic: boolean;
	    severity: string;
	
	    static createFrom(source: any = {}) {
	        return new IncidentCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = source["time"];
	        this.durationMs = source["durationMs"];
	        this.type = source["type"];
	        this.verdictText = source["verdictText"];
	        this.culprit = source["culprit"];
	        this.culpritPID = source["culpritPID"];
	        this.confidence = source["confidence"];
	        this.evidence = source["evidence"];
	        this.evidenceRaw = source["evidenceRaw"];
	        this.ecoTargets = this.convertValues(source["ecoTargets"], EcoTarget);
	        this.canEco = source["canEco"];
	        this.systemProc = source["systemProc"];
	        this.synthetic = source["synthetic"];
	        this.severity = source["severity"];
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
	export class DiagnosticReport {
	    generatedAt: string;
	    pulseCoreVersion: string;
	    windowsVersion: string;
	    arch: string;
	    cpuName: string;
	    logicalCPUs: number;
	    gpuName: string;
	    ramTotalMB: number;
	    ramUsedMB: number;
	    temperatureC: number;
	    tempAvailable: boolean;
	    gpuAvailable: boolean;
	    healthScore: number;
	    healthIncomplete: boolean;
	    incidents: IncidentCard[];
	    topCPU: ProcessSnap[];
	    topRAM: ProcessSnap[];
	    ecoHistoryCount: number;
	    ecoLimitedCount: number;
	    unavailableMetrics: string[];
	    collectorNotes: string[];
	    anomalyCount: number;
	    memoryWatchCount: number;
	    netConnType?: string;
	    netLocalIPMasked?: string;
	    netGatewayMasked?: string;
	    netInternetMs?: number;
	    netWifiSSIDMasked?: string;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generatedAt = source["generatedAt"];
	        this.pulseCoreVersion = source["pulseCoreVersion"];
	        this.windowsVersion = source["windowsVersion"];
	        this.arch = source["arch"];
	        this.cpuName = source["cpuName"];
	        this.logicalCPUs = source["logicalCPUs"];
	        this.gpuName = source["gpuName"];
	        this.ramTotalMB = source["ramTotalMB"];
	        this.ramUsedMB = source["ramUsedMB"];
	        this.temperatureC = source["temperatureC"];
	        this.tempAvailable = source["tempAvailable"];
	        this.gpuAvailable = source["gpuAvailable"];
	        this.healthScore = source["healthScore"];
	        this.healthIncomplete = source["healthIncomplete"];
	        this.incidents = this.convertValues(source["incidents"], IncidentCard);
	        this.topCPU = this.convertValues(source["topCPU"], ProcessSnap);
	        this.topRAM = this.convertValues(source["topRAM"], ProcessSnap);
	        this.ecoHistoryCount = source["ecoHistoryCount"];
	        this.ecoLimitedCount = source["ecoLimitedCount"];
	        this.unavailableMetrics = source["unavailableMetrics"];
	        this.collectorNotes = source["collectorNotes"];
	        this.anomalyCount = source["anomalyCount"];
	        this.memoryWatchCount = source["memoryWatchCount"];
	        this.netConnType = source["netConnType"];
	        this.netLocalIPMasked = source["netLocalIPMasked"];
	        this.netGatewayMasked = source["netGatewayMasked"];
	        this.netInternetMs = source["netInternetMs"];
	        this.netWifiSSIDMasked = source["netWifiSSIDMasked"];
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
	export class DiagnosticReportPreview {
	    includes: string[];
	    excludes: string[];
	    formats: string[];
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticReportPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.includes = source["includes"];
	        this.excludes = source["excludes"];
	        this.formats = source["formats"];
	    }
	}
	export class DiskFreeSnap {
	    mount: string;
	    freeGB: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskFreeSnap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mount = source["mount"];
	        this.freeGB = source["freeGB"];
	    }
	}
	export class SmartAttribute {
	    id: number;
	    name: string;
	    nameRu: string;
	    current: number;
	    worst: number;
	    threshold: number;
	    raw: string;
	    rawValue: number;
	
	    static createFrom(source: any = {}) {
	        return new SmartAttribute(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.nameRu = source["nameRu"];
	        this.current = source["current"];
	        this.worst = source["worst"];
	        this.threshold = source["threshold"];
	        this.raw = source["raw"];
	        this.rawValue = source["rawValue"];
	    }
	}
	export class DiskHealthReport {
	    driveLetter: string;
	    model: string;
	    serialNumber: string;
	    firmware: string;
	    mediaType: string;
	    mediaKind: string;
	    busInterface: string;
	    interfaceType: string;
	    rotationRPM: number;
	    capacityBytes: number;
	    capacityGB: number;
	    healthStatus: string;
	    operationalStatus: string;
	    statusLevel: string;
	    statusCode: string;
	    statusWordRu: string;
	    statusWordEn: string;
	    statusReason: loc.Msg;
	    statusReasonRu: string;
	    statusReasonEn: string;
	    infoNote: loc.Msg;
	    triggerAttrId: number;
	    triggerAttrHex: string;
	    profileId: string;
	    verdict: string;
	    verdictBadge: string;
	    healthPercent: number;
	    healthPercentOK: boolean;
	    healthSource: string;
	    healthSourceMsg: loc.Msg;
	    healthSourceRu: string;
	    healthSourceEn: string;
	    temperatureC: number;
	    tempAvailable: boolean;
	    wearRemaining: number;
	    powerOnHours: number;
	    powerOnHuman: loc.Msg;
	    powerOnHumanRu: string;
	    powerOnHumanEn: string;
	    powerCycles: number;
	    tbwGB: number;
	    hostWriteTB: number;
	    hostReadTB: number;
	    hostWriteGB: number;
	    hostReadGB: number;
	    reallocatedSectors: number;
	    pendingSectors: number;
	    uncorrectableSectors: number;
	    hasCriticalErrors: boolean;
	    readErrors: number;
	    writeErrors: number;
	    attributes: SmartAttribute[];
	    missingFieldsRu: string[];
	    missingFieldsEn: string[];
	    missingSummaryRu: string;
	    missingSummaryEn: string;
	    checkedAt: string;
	    diskIndex: number;
	    needsAdmin: boolean;
	    adminBannerRu: string;
	    adminBannerEn: string;
	    partialReasonRu: string;
	    partialReasonEn: string;
	    fromCache: boolean;
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
	        this.firmware = source["firmware"];
	        this.mediaType = source["mediaType"];
	        this.mediaKind = source["mediaKind"];
	        this.busInterface = source["busInterface"];
	        this.interfaceType = source["interfaceType"];
	        this.rotationRPM = source["rotationRPM"];
	        this.capacityBytes = source["capacityBytes"];
	        this.capacityGB = source["capacityGB"];
	        this.healthStatus = source["healthStatus"];
	        this.operationalStatus = source["operationalStatus"];
	        this.statusLevel = source["statusLevel"];
	        this.statusCode = source["statusCode"];
	        this.statusWordRu = source["statusWordRu"];
	        this.statusWordEn = source["statusWordEn"];
	        this.statusReason = this.convertValues(source["statusReason"], loc.Msg);
	        this.statusReasonRu = source["statusReasonRu"];
	        this.statusReasonEn = source["statusReasonEn"];
	        this.infoNote = this.convertValues(source["infoNote"], loc.Msg);
	        this.triggerAttrId = source["triggerAttrId"];
	        this.triggerAttrHex = source["triggerAttrHex"];
	        this.profileId = source["profileId"];
	        this.verdict = source["verdict"];
	        this.verdictBadge = source["verdictBadge"];
	        this.healthPercent = source["healthPercent"];
	        this.healthPercentOK = source["healthPercentOK"];
	        this.healthSource = source["healthSource"];
	        this.healthSourceMsg = this.convertValues(source["healthSourceMsg"], loc.Msg);
	        this.healthSourceRu = source["healthSourceRu"];
	        this.healthSourceEn = source["healthSourceEn"];
	        this.temperatureC = source["temperatureC"];
	        this.tempAvailable = source["tempAvailable"];
	        this.wearRemaining = source["wearRemaining"];
	        this.powerOnHours = source["powerOnHours"];
	        this.powerOnHuman = this.convertValues(source["powerOnHuman"], loc.Msg);
	        this.powerOnHumanRu = source["powerOnHumanRu"];
	        this.powerOnHumanEn = source["powerOnHumanEn"];
	        this.powerCycles = source["powerCycles"];
	        this.tbwGB = source["tbwGB"];
	        this.hostWriteTB = source["hostWriteTB"];
	        this.hostReadTB = source["hostReadTB"];
	        this.hostWriteGB = source["hostWriteGB"];
	        this.hostReadGB = source["hostReadGB"];
	        this.reallocatedSectors = source["reallocatedSectors"];
	        this.pendingSectors = source["pendingSectors"];
	        this.uncorrectableSectors = source["uncorrectableSectors"];
	        this.hasCriticalErrors = source["hasCriticalErrors"];
	        this.readErrors = source["readErrors"];
	        this.writeErrors = source["writeErrors"];
	        this.attributes = this.convertValues(source["attributes"], SmartAttribute);
	        this.missingFieldsRu = source["missingFieldsRu"];
	        this.missingFieldsEn = source["missingFieldsEn"];
	        this.missingSummaryRu = source["missingSummaryRu"];
	        this.missingSummaryEn = source["missingSummaryEn"];
	        this.checkedAt = source["checkedAt"];
	        this.diskIndex = source["diskIndex"];
	        this.needsAdmin = source["needsAdmin"];
	        this.adminBannerRu = source["adminBannerRu"];
	        this.adminBannerEn = source["adminBannerEn"];
	        this.partialReasonRu = source["partialReasonRu"];
	        this.partialReasonEn = source["partialReasonEn"];
	        this.fromCache = source["fromCache"];
	        this.errorMessage = source["errorMessage"];
	        this.ok = source["ok"];
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
	export class DiskWatchInfo {
	    letter: string;
	    watching: boolean;
	    trendCode: string;
	    from: number;
	    to: number;
	    days: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskWatchInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.letter = source["letter"];
	        this.watching = source["watching"];
	        this.trendCode = source["trendCode"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.days = source["days"];
	    }
	}
	export class DisplayInfo {
	    deviceName: string;
	    monitorName: string;
	    adapterName: string;
	    isPrimary: boolean;
	    currentWidth: number;
	    currentHeight: number;
	    width: number;
	    height: number;
	    currentHz: number;
	    maxHz: number;
	    maxSupportedHz: number;
	    availableHz: number[];
	    needsAttention: boolean;
	    underclocked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DisplayInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceName = source["deviceName"];
	        this.monitorName = source["monitorName"];
	        this.adapterName = source["adapterName"];
	        this.isPrimary = source["isPrimary"];
	        this.currentWidth = source["currentWidth"];
	        this.currentHeight = source["currentHeight"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.currentHz = source["currentHz"];
	        this.maxHz = source["maxHz"];
	        this.maxSupportedHz = source["maxSupportedHz"];
	        this.availableHz = source["availableHz"];
	        this.needsAttention = source["needsAttention"];
	        this.underclocked = source["underclocked"];
	    }
	}
	export class EcoFail {
	    name: string;
	    pid: number;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new EcoFail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.pid = source["pid"];
	        this.reason = source["reason"];
	    }
	}
	export class EcoApplyResult {
	    ok: boolean;
	    appliedCount: number;
	    failed: EcoFail[];
	    confirmed: boolean;
	    messageRu: string;
	    messageEn: string;
	    cpuBefore: number;
	    cpuAfter: number;
	    ramBeforeMB: number;
	    ramAfterMB: number;
	    effectNoted: boolean;
	    canRevert: boolean;
	    sessionId: string;
	    softEffect: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EcoApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.appliedCount = source["appliedCount"];
	        this.failed = this.convertValues(source["failed"], EcoFail);
	        this.confirmed = source["confirmed"];
	        this.messageRu = source["messageRu"];
	        this.messageEn = source["messageEn"];
	        this.cpuBefore = source["cpuBefore"];
	        this.cpuAfter = source["cpuAfter"];
	        this.ramBeforeMB = source["ramBeforeMB"];
	        this.ramAfterMB = source["ramAfterMB"];
	        this.effectNoted = source["effectNoted"];
	        this.canRevert = source["canRevert"];
	        this.sessionId = source["sessionId"];
	        this.softEffect = source["softEffect"];
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
	
	export class EcoLimitedGroup {
	    name: string;
	    friendlyName: string;
	    count: number;
	    pids: number[];
	    appliedAt: string;
	    sessionId: string;
	
	    static createFrom(source: any = {}) {
	        return new EcoLimitedGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.friendlyName = source["friendlyName"];
	        this.count = source["count"];
	        this.pids = source["pids"];
	        this.appliedAt = source["appliedAt"];
	        this.sessionId = source["sessionId"];
	    }
	}
	
	export class GameFitVerdict {
	    level: string;
	    labelRu: string;
	    labelEn: string;
	    reasonRu: string;
	    reasonEn: string;
	    approximate: boolean;
	    confident: boolean;
	    storageNeedGB: number;
	    storageNeedOK: boolean;
	    disks: DiskFreeSnap[];
	    recommendMount: string;
	    downloadRu: string;
	    downloadEn: string;
	
	    static createFrom(source: any = {}) {
	        return new GameFitVerdict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.labelRu = source["labelRu"];
	        this.labelEn = source["labelEn"];
	        this.reasonRu = source["reasonRu"];
	        this.reasonEn = source["reasonEn"];
	        this.approximate = source["approximate"];
	        this.confident = source["confident"];
	        this.storageNeedGB = source["storageNeedGB"];
	        this.storageNeedOK = source["storageNeedOK"];
	        this.disks = this.convertValues(source["disks"], DiskFreeSnap);
	        this.recommendMount = source["recommendMount"];
	        this.downloadRu = source["downloadRu"];
	        this.downloadEn = source["downloadEn"];
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
	    sourceNote: string;
	    archConfirm: string;
	    upscaleHint?: string;
	    fit: GameFitVerdict;
	    fitVerdict?: string;
	    fitReasonRu?: string;
	    fitReasonEn?: string;
	    reqVRAMGB: number;
	    reqRAMGB: number;
	
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
	        this.sourceNote = source["sourceNote"];
	        this.archConfirm = source["archConfirm"];
	        this.upscaleHint = source["upscaleHint"];
	        this.fit = this.convertValues(source["fit"], GameFitVerdict);
	        this.fitVerdict = source["fitVerdict"];
	        this.fitReasonRu = source["fitReasonRu"];
	        this.fitReasonEn = source["fitReasonEn"];
	        this.reqVRAMGB = source["reqVRAMGB"];
	        this.reqRAMGB = source["reqRAMGB"];
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
	export class GPUDeepInfo {
	    name: string;
	    driverVersion: string;
	    dedicatedMB: number;
	    sharedMB: number;
	    usedMB: number;
	    percent: number;
	    isIntegrated: boolean;
	    vramGB: number;
	    temperatureC: number;
	    tempAvailable: boolean;
	    powerW: number;
	    powerAvailable: boolean;
	    clockCoreMHz: number;
	    clockMemMHz: number;
	
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
	        this.temperatureC = source["temperatureC"];
	        this.tempAvailable = source["tempAvailable"];
	        this.powerW = source["powerW"];
	        this.powerAvailable = source["powerAvailable"];
	        this.clockCoreMHz = source["clockCoreMHz"];
	        this.clockMemMHz = source["clockMemMHz"];
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
	    installPath?: string;
	    installSizeGB?: number;
	    minVramGB?: number;
	    recVramGB?: number;
	    minRAMGB?: number;
	    recRAMGB?: number;
	    fitVerdict?: string;
	    fitVerdictCls?: string;
	    fitLabelRu?: string;
	    fitLabelEn?: string;
	    fitReasonRu?: string;
	    fitReasonEn?: string;
	
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
	        this.installPath = source["installPath"];
	        this.installSizeGB = source["installSizeGB"];
	        this.minVramGB = source["minVramGB"];
	        this.recVramGB = source["recVramGB"];
	        this.minRAMGB = source["minRAMGB"];
	        this.recRAMGB = source["recRAMGB"];
	        this.fitVerdict = source["fitVerdict"];
	        this.fitVerdictCls = source["fitVerdictCls"];
	        this.fitLabelRu = source["fitLabelRu"];
	        this.fitLabelEn = source["fitLabelEn"];
	        this.fitReasonRu = source["fitReasonRu"];
	        this.fitReasonEn = source["fitReasonEn"];
	    }
	}
	export class GameSession {
	    id: string;
	    name: string;
	    exe: string;
	    start: string;
	    end?: string;
	    durationMin: number;
	    cpuMax: number;
	    cpuAvg: number;
	    gpuMax: number;
	    gpuAvg: number;
	    ramMax: number;
	    tempCPUMax: number;
	    tempCPUAvg: number;
	    tempGPUMax: number;
	    tempGPUAvg: number;
	    freezes: number;
	    verdict: string;
	    active: boolean;
	    synthetic?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GameSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.exe = source["exe"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.durationMin = source["durationMin"];
	        this.cpuMax = source["cpuMax"];
	        this.cpuAvg = source["cpuAvg"];
	        this.gpuMax = source["gpuMax"];
	        this.gpuAvg = source["gpuAvg"];
	        this.ramMax = source["ramMax"];
	        this.tempCPUMax = source["tempCPUMax"];
	        this.tempCPUAvg = source["tempCPUAvg"];
	        this.tempGPUMax = source["tempGPUMax"];
	        this.tempGPUAvg = source["tempGPUAvg"];
	        this.freezes = source["freezes"];
	        this.verdict = source["verdict"];
	        this.active = source["active"];
	        this.synthetic = source["synthetic"];
	    }
	}
	export class GameSessionCompare {
	    ok: boolean;
	    left: GameSession;
	    right: GameSession;
	    notes: string[];
	
	    static createFrom(source: any = {}) {
	        return new GameSessionCompare(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.left = this.convertValues(source["left"], GameSession);
	        this.right = this.convertValues(source["right"], GameSession);
	        this.notes = source["notes"];
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
	    powerScheme?: string;
	    cpuWatts?: number;
	    gpuWatts?: number;
	    totalWatts?: number;
	
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
	        this.powerScheme = source["powerScheme"];
	        this.cpuWatts = source["cpuWatts"];
	        this.gpuWatts = source["gpuWatts"];
	        this.totalWatts = source["totalWatts"];
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
	    moduleModel?: string;
	    partNumber?: string;
	    manufacturer?: string;
	
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
	        this.moduleModel = source["moduleModel"];
	        this.partNumber = source["partNumber"];
	        this.manufacturer = source["manufacturer"];
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
	export class HealthDeduction {
	    key: string;
	    labelRu: string;
	    labelEn: string;
	    points: number;
	    detailRu: string;
	    detailEn: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthDeduction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.labelRu = source["labelRu"];
	        this.labelEn = source["labelEn"];
	        this.points = source["points"];
	        this.detailRu = source["detailRu"];
	        this.detailEn = source["detailEn"];
	    }
	}
	export class HealthFactor {
	    key: string;
	    labelRu: string;
	    labelEn: string;
	    score: number;
	    weight: number;
	    available: boolean;
	    status: string;
	    detailRu: string;
	    detailEn: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthFactor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.labelRu = source["labelRu"];
	        this.labelEn = source["labelEn"];
	        this.score = source["score"];
	        this.weight = source["weight"];
	        this.available = source["available"];
	        this.status = source["status"];
	        this.detailRu = source["detailRu"];
	        this.detailEn = source["detailEn"];
	    }
	}
	export class HealthHistoryPoint {
	    at: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new HealthHistoryPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = source["at"];
	        this.score = source["score"];
	    }
	}
	export class HiddenGamePref {
	    id: string;
	    name?: string;
	    posterUrl?: string;
	    year?: number;
	
	    static createFrom(source: any = {}) {
	        return new HiddenGamePref(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.posterUrl = source["posterUrl"];
	        this.year = source["year"];
	    }
	}
	export class HomeActionResult {
	    ok: boolean;
	    message: loc.Msg;
	    scoreBefore: number;
	    scoreAfter: number;
	    freedRAMMB: number;
	    canRevert: boolean;
	    openTab: string;
	
	    static createFrom(source: any = {}) {
	        return new HomeActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = this.convertValues(source["message"], loc.Msg);
	        this.scoreBefore = source["scoreBefore"];
	        this.scoreAfter = source["scoreAfter"];
	        this.freedRAMMB = source["freedRAMMB"];
	        this.canRevert = source["canRevert"];
	        this.openTab = source["openTab"];
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
	export class HomeProblem {
	    id: string;
	    priority: number;
	    title: loc.Msg;
	    why: loc.Msg;
	    actionId: string;
	    action: loc.Msg;
	    actionArg: string;
	    gainEstimate: number;
	    gainRaw: number;
	    gainUpTo: boolean;
	    detail: loc.Msg;
	    statusLevel: string;
	
	    static createFrom(source: any = {}) {
	        return new HomeProblem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.priority = source["priority"];
	        this.title = this.convertValues(source["title"], loc.Msg);
	        this.why = this.convertValues(source["why"], loc.Msg);
	        this.actionId = source["actionId"];
	        this.action = this.convertValues(source["action"], loc.Msg);
	        this.actionArg = source["actionArg"];
	        this.gainEstimate = source["gainEstimate"];
	        this.gainRaw = source["gainRaw"];
	        this.gainUpTo = source["gainUpTo"];
	        this.detail = this.convertValues(source["detail"], loc.Msg);
	        this.statusLevel = source["statusLevel"];
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
	export class HomeState {
	    score: number;
	    scoreIfAllFixed: number;
	    status: loc.Msg;
	    checkedAgoSec: number;
	    problems: HomeProblem[];
	    allGood: boolean;
	    incomplete: boolean;
	    missingNote: string;
	    deductions: HealthDeduction[];
	    prepareActive: boolean;
	    prepareRAMMB: number;
	    ecoLimitedCount: number;
	    powerMode: string;
	
	    static createFrom(source: any = {}) {
	        return new HomeState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = source["score"];
	        this.scoreIfAllFixed = source["scoreIfAllFixed"];
	        this.status = this.convertValues(source["status"], loc.Msg);
	        this.checkedAgoSec = source["checkedAgoSec"];
	        this.problems = this.convertValues(source["problems"], HomeProblem);
	        this.allGood = source["allGood"];
	        this.incomplete = source["incomplete"];
	        this.missingNote = source["missingNote"];
	        this.deductions = this.convertValues(source["deductions"], HealthDeduction);
	        this.prepareActive = source["prepareActive"];
	        this.prepareRAMMB = source["prepareRAMMB"];
	        this.ecoLimitedCount = source["ecoLimitedCount"];
	        this.powerMode = source["powerMode"];
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
	
	export class InstalledApp {
	    id: string;
	    displayName: string;
	    displayVersion: string;
	    publisher: string;
	    installLocation: string;
	    sizeBytes: number;
	    sizeLabel: string;
	    uninstallString: string;
	    quietUninstallString: string;
	    canUninstall: boolean;
	    registryRoot: string;
	    registryPath: string;
	    driveLetter: string;
	    lastUsedDays: number;
	    candidateCleanup: boolean;
	    candidateBadgeRu?: string;
	    candidateBadgeEn?: string;
	
	    static createFrom(source: any = {}) {
	        return new InstalledApp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.displayVersion = source["displayVersion"];
	        this.publisher = source["publisher"];
	        this.installLocation = source["installLocation"];
	        this.sizeBytes = source["sizeBytes"];
	        this.sizeLabel = source["sizeLabel"];
	        this.uninstallString = source["uninstallString"];
	        this.quietUninstallString = source["quietUninstallString"];
	        this.canUninstall = source["canUninstall"];
	        this.registryRoot = source["registryRoot"];
	        this.registryPath = source["registryPath"];
	        this.driveLetter = source["driveLetter"];
	        this.lastUsedDays = source["lastUsedDays"];
	        this.candidateCleanup = source["candidateCleanup"];
	        this.candidateBadgeRu = source["candidateBadgeRu"];
	        this.candidateBadgeEn = source["candidateBadgeEn"];
	    }
	}
	export class IntelBaseline {
	    scenario: string;
	    cpu: number;
	    ram: number;
	    gpu: number;
	    tempCPU: number;
	    samples: number;
	    ready: boolean;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new IntelBaseline(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scenario = source["scenario"];
	        this.cpu = source["cpu"];
	        this.ram = source["ram"];
	        this.gpu = source["gpu"];
	        this.tempCPU = source["tempCPU"];
	        this.samples = source["samples"];
	        this.ready = source["ready"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class IntelInsight {
	    id: string;
	    scenario: string;
	    metric: string;
	    current: number;
	    typical: number;
	    delta: number;
	    confidence: number;
	    textRu: string;
	    textEn: string;
	    detectedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new IntelInsight(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.scenario = source["scenario"];
	        this.metric = source["metric"];
	        this.current = source["current"];
	        this.typical = source["typical"];
	        this.delta = source["delta"];
	        this.confidence = source["confidence"];
	        this.textRu = source["textRu"];
	        this.textEn = source["textEn"];
	        this.detectedAt = source["detectedAt"];
	    }
	}
	export class IntelligenceStatus {
	    ok: boolean;
	    minSamples: number;
	    observingDays: number;
	    baselines: IntelBaseline[];
	    insights: IntelInsight[];
	    noteRu: string;
	    noteEn: string;
	
	    static createFrom(source: any = {}) {
	        return new IntelligenceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.minSamples = source["minSamples"];
	        this.observingDays = source["observingDays"];
	        this.baselines = this.convertValues(source["baselines"], IntelBaseline);
	        this.insights = this.convertValues(source["insights"], IntelInsight);
	        this.noteRu = source["noteRu"];
	        this.noteEn = source["noteEn"];
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
	export class LargeFolder {
	    path: string;
	    sizeBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new LargeFolder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.sizeBytes = source["sizeBytes"];
	    }
	}
	export class LocalDriveInfo {
	    letter: string;
	    mount: string;
	    label: string;
	    driveType: string;
	    totalBytes: number;
	    freeBytes: number;
	    usedBytes: number;
	    totalGB: number;
	    freeGB: number;
	    usedGB: number;
	    usedPercent: number;
	
	    static createFrom(source: any = {}) {
	        return new LocalDriveInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.letter = source["letter"];
	        this.mount = source["mount"];
	        this.label = source["label"];
	        this.driveType = source["driveType"];
	        this.totalBytes = source["totalBytes"];
	        this.freeBytes = source["freeBytes"];
	        this.usedBytes = source["usedBytes"];
	        this.totalGB = source["totalGB"];
	        this.freeGB = source["freeGB"];
	        this.usedGB = source["usedGB"];
	        this.usedPercent = source["usedPercent"];
	    }
	}
	export class MemoryWatchCard {
	    process: string;
	    friendly: string;
	    fromMB: number;
	    toMB: number;
	    minutes: number;
	    verdict: string;
	    textRu: string;
	    textEn: string;
	    confidence: number;
	    detectedAt: string;
	    pid: number;
	
	    static createFrom(source: any = {}) {
	        return new MemoryWatchCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.process = source["process"];
	        this.friendly = source["friendly"];
	        this.fromMB = source["fromMB"];
	        this.toMB = source["toMB"];
	        this.minutes = source["minutes"];
	        this.verdict = source["verdict"];
	        this.textRu = source["textRu"];
	        this.textEn = source["textEn"];
	        this.confidence = source["confidence"];
	        this.detectedAt = source["detectedAt"];
	        this.pid = source["pid"];
	    }
	}
	export class MonthlyAppReport {
	    ok: boolean;
	    period: string;
	    topApps: AppUsageEntry[];
	    note: string;
	    warmingUp: boolean;
	    sessionSecs: number;
	
	    static createFrom(source: any = {}) {
	        return new MonthlyAppReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.period = source["period"];
	        this.topApps = this.convertValues(source["topApps"], AppUsageEntry);
	        this.note = source["note"];
	        this.warmingUp = source["warmingUp"];
	        this.sessionSecs = source["sessionSecs"];
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
	export class NetworkDetail {
	    ok: boolean;
	    checked: boolean;
	    cached: boolean;
	    cancelled: boolean;
	    verdictKey: string;
	    level: string;
	    summaryPingMs: number;
	    connType?: string;
	    localIP?: string;
	    gateway?: string;
	    gatewayMs: number;
	    dnsServers?: string[];
	    dnsMs: number;
	    internetHost?: string;
	    internetMs: number;
	    adapterName?: string;
	    adapterStatus?: string;
	    linkMbps?: number;
	    linkSpeedLabel?: string;
	    wifiSignal?: number;
	    wifiBand?: string;
	    wifiSSID?: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.checked = source["checked"];
	        this.cached = source["cached"];
	        this.cancelled = source["cancelled"];
	        this.verdictKey = source["verdictKey"];
	        this.level = source["level"];
	        this.summaryPingMs = source["summaryPingMs"];
	        this.connType = source["connType"];
	        this.localIP = source["localIP"];
	        this.gateway = source["gateway"];
	        this.gatewayMs = source["gatewayMs"];
	        this.dnsServers = source["dnsServers"];
	        this.dnsMs = source["dnsMs"];
	        this.internetHost = source["internetHost"];
	        this.internetMs = source["internetMs"];
	        this.adapterName = source["adapterName"];
	        this.adapterStatus = source["adapterStatus"];
	        this.linkMbps = source["linkMbps"];
	        this.linkSpeedLabel = source["linkSpeedLabel"];
	        this.wifiSignal = source["wifiSignal"];
	        this.wifiBand = source["wifiBand"];
	        this.wifiSSID = source["wifiSSID"];
	    }
	}
	export class ProcessNetEntry {
	    name: string;
	    friendly: string;
	    pid: number;
	    sendKBps: number;
	    recvKBps: number;
	    sendMB: number;
	    recvMB: number;
	    connCount: number;
	    ratesKnown: boolean;
	    detailRu: string;
	    detailEn: string;
	
	    static createFrom(source: any = {}) {
	        return new ProcessNetEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.friendly = source["friendly"];
	        this.pid = source["pid"];
	        this.sendKBps = source["sendKBps"];
	        this.recvKBps = source["recvKBps"];
	        this.sendMB = source["sendMB"];
	        this.recvMB = source["recvMB"];
	        this.connCount = source["connCount"];
	        this.ratesKnown = source["ratesKnown"];
	        this.detailRu = source["detailRu"];
	        this.detailEn = source["detailEn"];
	    }
	}
	export class NetworkMonitorSnapshot {
	    ok: boolean;
	    systemSendKBps: number;
	    systemRecvKBps: number;
	    sessionSendMB: number;
	    sessionRecvMB: number;
	    perProcessOK: boolean;
	    perProcessNoteRu: string;
	    perProcessNoteEn: string;
	    processes: ProcessNetEntry[];
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkMonitorSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.systemSendKBps = source["systemSendKBps"];
	        this.systemRecvKBps = source["systemRecvKBps"];
	        this.sessionSendMB = source["sessionSendMB"];
	        this.sessionRecvMB = source["sessionRecvMB"];
	        this.perProcessOK = source["perProcessOK"];
	        this.perProcessNoteRu = source["perProcessNoteRu"];
	        this.perProcessNoteEn = source["perProcessNoteEn"];
	        this.processes = this.convertValues(source["processes"], ProcessNetEntry);
	        this.updatedAt = source["updatedAt"];
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
	export class OrphanLeftover {
	    path: string;
	    name: string;
	    sizeBytes: number;
	    sizeLabel: string;
	    rootKind: string;
	    lastModTime: time.Time;
	    daysAgo: number;
	    ageBadgeRu: string;
	    ageBadgeEn: string;
	    ageStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new OrphanLeftover(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.sizeBytes = source["sizeBytes"];
	        this.sizeLabel = source["sizeLabel"];
	        this.rootKind = source["rootKind"];
	        this.lastModTime = this.convertValues(source["lastModTime"], time.Time);
	        this.daysAgo = source["daysAgo"];
	        this.ageBadgeRu = source["ageBadgeRu"];
	        this.ageBadgeEn = source["ageBadgeEn"];
	        this.ageStatus = source["ageStatus"];
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
	export class PnPDeviceError {
	    name: string;
	    deviceId: string;
	    errorCode: number;
	    statusDescription: string;
	    explainCode: string;
	    explain: loc.Msg;
	    actionId: string;
	    actionLabelCode: string;
	    action: loc.Msg;
	
	    static createFrom(source: any = {}) {
	        return new PnPDeviceError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.deviceId = source["deviceId"];
	        this.errorCode = source["errorCode"];
	        this.statusDescription = source["statusDescription"];
	        this.explainCode = source["explainCode"];
	        this.explain = this.convertValues(source["explain"], loc.Msg);
	        this.actionId = source["actionId"];
	        this.actionLabelCode = source["actionLabelCode"];
	        this.action = this.convertValues(source["action"], loc.Msg);
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
	
	export class ProcessAnomaly {
	    id: string;
	    process: string;
	    friendly: string;
	    type: string;
	    current: number;
	    baseline: number;
	    deltaPct: number;
	    unit: string;
	    detectedAt: string;
	    detectedUnix: number;
	    confidence: number;
	    textRu: string;
	    textEn: string;
	    scenario: string;
	    hidden: boolean;
	    expiresUnix: number;
	
	    static createFrom(source: any = {}) {
	        return new ProcessAnomaly(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.process = source["process"];
	        this.friendly = source["friendly"];
	        this.type = source["type"];
	        this.current = source["current"];
	        this.baseline = source["baseline"];
	        this.deltaPct = source["deltaPct"];
	        this.unit = source["unit"];
	        this.detectedAt = source["detectedAt"];
	        this.detectedUnix = source["detectedUnix"];
	        this.confidence = source["confidence"];
	        this.textRu = source["textRu"];
	        this.textEn = source["textEn"];
	        this.scenario = source["scenario"];
	        this.hidden = source["hidden"];
	        this.expiresUnix = source["expiresUnix"];
	    }
	}
	
	
	
	export class RecycleBinStatus {
	    bytes: number;
	    items: number;
	
	    static createFrom(source: any = {}) {
	        return new RecycleBinStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bytes = source["bytes"];
	        this.items = source["items"];
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
	
	export class SmartFixAppliedStep {
	    stepId: string;
	    scenarioId: string;
	    ok: boolean;
	    reversible: boolean;
	    title: loc.Msg;
	    detail: loc.Msg;
	    openTab?: string;
	
	    static createFrom(source: any = {}) {
	        return new SmartFixAppliedStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stepId = source["stepId"];
	        this.scenarioId = source["scenarioId"];
	        this.ok = source["ok"];
	        this.reversible = source["reversible"];
	        this.title = this.convertValues(source["title"], loc.Msg);
	        this.detail = this.convertValues(source["detail"], loc.Msg);
	        this.openTab = source["openTab"];
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
	export class SmartFixJournalStep {
	    stepId: string;
	    scenarioId: string;
	    actionId: string;
	    actionArg: string;
	    reversible: boolean;
	    kind: string;
	    prevJson: string;
	    afterJson?: string;
	    ok: boolean;
	    detail?: string;
	
	    static createFrom(source: any = {}) {
	        return new SmartFixJournalStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stepId = source["stepId"];
	        this.scenarioId = source["scenarioId"];
	        this.actionId = source["actionId"];
	        this.actionArg = source["actionArg"];
	        this.reversible = source["reversible"];
	        this.kind = source["kind"];
	        this.prevJson = source["prevJson"];
	        this.afterJson = source["afterJson"];
	        this.ok = source["ok"];
	        this.detail = source["detail"];
	    }
	}
	export class SmartFixJournalSession {
	    id: string;
	    at: string;
	    scoreBefore: number;
	    scoreAfter: number;
	    freedBytes: number;
	    freedRAMMB: number;
	    steps: SmartFixJournalStep[];
	    reverted: boolean;
	    revertedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new SmartFixJournalSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.at = source["at"];
	        this.scoreBefore = source["scoreBefore"];
	        this.scoreAfter = source["scoreAfter"];
	        this.freedBytes = source["freedBytes"];
	        this.freedRAMMB = source["freedRAMMB"];
	        this.steps = this.convertValues(source["steps"], SmartFixJournalStep);
	        this.reverted = source["reverted"];
	        this.revertedAt = source["revertedAt"];
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
	
	export class StartupItem {
	    id: string;
	    name: string;
	    command: string;
	    path: string;
	    publisher: string;
	    location: string;
	    locationRu: string;
	    locationEn: string;
	    enabled: boolean;
	    canDisable: boolean;
	    critical: boolean;
	    impact?: string;
	    impactRu?: string;
	    impactEn?: string;
	    detailRu?: string;
	    detailEn?: string;
	    advise: string;
	    category?: string;
	    categoryKey?: string;
	    categoryEstimate?: boolean;
	    running?: boolean;
	    cpuPercent?: number;
	    ramMB?: number;
	
	    static createFrom(source: any = {}) {
	        return new StartupItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.command = source["command"];
	        this.path = source["path"];
	        this.publisher = source["publisher"];
	        this.location = source["location"];
	        this.locationRu = source["locationRu"];
	        this.locationEn = source["locationEn"];
	        this.enabled = source["enabled"];
	        this.canDisable = source["canDisable"];
	        this.critical = source["critical"];
	        this.impact = source["impact"];
	        this.impactRu = source["impactRu"];
	        this.impactEn = source["impactEn"];
	        this.detailRu = source["detailRu"];
	        this.detailEn = source["detailEn"];
	        this.advise = source["advise"];
	        this.category = source["category"];
	        this.categoryKey = source["categoryKey"];
	        this.categoryEstimate = source["categoryEstimate"];
	        this.running = source["running"];
	        this.cpuPercent = source["cpuPercent"];
	        this.ramMB = source["ramMB"];
	    }
	}
	export class SmartFixStep {
	    id: string;
	    scenarioId: string;
	    title: loc.Msg;
	    why: loc.Msg;
	    gainEstimate: number;
	    gainRaw: number;
	    gainUpTo: boolean;
	    reversible: boolean;
	    irreversible: boolean;
	    needsAdmin: boolean;
	    selected: boolean;
	    actionId: string;
	    actionArg: string;
	    openUrl: string;
	    instruction: loc.Msg;
	    priority: number;
	
	    static createFrom(source: any = {}) {
	        return new SmartFixStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.scenarioId = source["scenarioId"];
	        this.title = this.convertValues(source["title"], loc.Msg);
	        this.why = this.convertValues(source["why"], loc.Msg);
	        this.gainEstimate = source["gainEstimate"];
	        this.gainRaw = source["gainRaw"];
	        this.gainUpTo = source["gainUpTo"];
	        this.reversible = source["reversible"];
	        this.irreversible = source["irreversible"];
	        this.needsAdmin = source["needsAdmin"];
	        this.selected = source["selected"];
	        this.actionId = source["actionId"];
	        this.actionArg = source["actionArg"];
	        this.openUrl = source["openUrl"];
	        this.instruction = this.convertValues(source["instruction"], loc.Msg);
	        this.priority = source["priority"];
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
	export class SmartFixPlan {
	    steps: SmartFixStep[];
	    scoreNow: number;
	    scoreIfApplied: number;
	    totalGain: number;
	    cleanupPreview: CleanupCat[];
	    hasCleanup: boolean;
	    devices: PnPDeviceError[];
	    displays: DisplayInfo[];
	    startupCandidates: StartupItem[];
	
	    static createFrom(source: any = {}) {
	        return new SmartFixPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.steps = this.convertValues(source["steps"], SmartFixStep);
	        this.scoreNow = source["scoreNow"];
	        this.scoreIfApplied = source["scoreIfApplied"];
	        this.totalGain = source["totalGain"];
	        this.cleanupPreview = this.convertValues(source["cleanupPreview"], CleanupCat);
	        this.hasCleanup = source["hasCleanup"];
	        this.devices = this.convertValues(source["devices"], PnPDeviceError);
	        this.displays = this.convertValues(source["displays"], DisplayInfo);
	        this.startupCandidates = this.convertValues(source["startupCandidates"], StartupItem);
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
	export class SmartFixResult {
	    ok: boolean;
	    sessionId: string;
	    scoreBefore: number;
	    scoreAfter: number;
	    freedBytes: number;
	    freedRAMMB: number;
	    applied: SmartFixAppliedStep[];
	    canRevert: boolean;
	    message: loc.Msg;
	    openTab: string;
	
	    static createFrom(source: any = {}) {
	        return new SmartFixResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.sessionId = source["sessionId"];
	        this.scoreBefore = source["scoreBefore"];
	        this.scoreAfter = source["scoreAfter"];
	        this.freedBytes = source["freedBytes"];
	        this.freedRAMMB = source["freedRAMMB"];
	        this.applied = this.convertValues(source["applied"], SmartFixAppliedStep);
	        this.canRevert = source["canRevert"];
	        this.message = this.convertValues(source["message"], loc.Msg);
	        this.openTab = source["openTab"];
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
	
	export class StartupDisableResult {
	    ok: boolean;
	    messageRu: string;
	    messageEn: string;
	
	    static createFrom(source: any = {}) {
	        return new StartupDisableResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.messageRu = source["messageRu"];
	        this.messageEn = source["messageEn"];
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
	export class SystemHealthReport {
	    score: number;
	    incomplete: boolean;
	    gradeRu: string;
	    gradeEn: string;
	    summaryRu: string;
	    summaryEn: string;
	    factors: HealthFactor[];
	    deductions: HealthDeduction[];
	    ok: string[];
	    watch: string[];
	    action: string[];
	    history: HealthHistoryPoint[];
	    computedAt: string;
	    missingNote: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemHealthReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = source["score"];
	        this.incomplete = source["incomplete"];
	        this.gradeRu = source["gradeRu"];
	        this.gradeEn = source["gradeEn"];
	        this.summaryRu = source["summaryRu"];
	        this.summaryEn = source["summaryEn"];
	        this.factors = this.convertValues(source["factors"], HealthFactor);
	        this.deductions = this.convertValues(source["deductions"], HealthDeduction);
	        this.ok = source["ok"];
	        this.watch = source["watch"];
	        this.action = source["action"];
	        this.history = this.convertValues(source["history"], HealthHistoryPoint);
	        this.computedAt = source["computedAt"];
	        this.missingNote = source["missingNote"];
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
	    reasonRu: string;
	    reasonEn: string;
	
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
	        this.reasonRu = source["reasonRu"];
	        this.reasonEn = source["reasonEn"];
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
	    temperatureAvailable: boolean;
	    temperatureSource: string;
	    temperatureSourceLabelRu: string;
	    temperatureSourceLabelEn: string;
	    gpuTemperatureC: number;
	    gpuTemperatureAvailable: boolean;
	    gpuPowerW: number;
	    gpuPowerAvailable: boolean;
	    gpuClockCoreMHz: number;
	    gpuClockMemMHz: number;
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
	        this.temperatureAvailable = source["temperatureAvailable"];
	        this.temperatureSource = source["temperatureSource"];
	        this.temperatureSourceLabelRu = source["temperatureSourceLabelRu"];
	        this.temperatureSourceLabelEn = source["temperatureSourceLabelEn"];
	        this.gpuTemperatureC = source["gpuTemperatureC"];
	        this.gpuTemperatureAvailable = source["gpuTemperatureAvailable"];
	        this.gpuPowerW = source["gpuPowerW"];
	        this.gpuPowerAvailable = source["gpuPowerAvailable"];
	        this.gpuClockCoreMHz = source["gpuClockCoreMHz"];
	        this.gpuClockMemMHz = source["gpuClockMemMHz"];
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
	export class UninstallAppResult {
	    ok: boolean;
	    message: string;
	    leftovers: AppLeftover[];
	    leftoverBytes: number;
	    freedBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new UninstallAppResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.leftovers = this.convertValues(source["leftovers"], AppLeftover);
	        this.leftoverBytes = source["leftoverBytes"];
	        this.freedBytes = source["freedBytes"];
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
	
	export class WeeklySummary {
	    ok: boolean;
	    insufficient: boolean;
	    message: loc.Msg;
	    messageRu: string;
	    messageEn: string;
	    gameHours: number;
	    prevGameHours: number;
	    freezes: number;
	    prevFreezes: number;
	    cpuTempPeak: number;
	    prevCPUTempPeak: number;
	    gpuTempPeak: number;
	    prevGPUTempPeak: number;
	    stableDay: string;
	    topCulprit: string;
	    ecoRAMFreedMB: number;
	    prevEcoRAMMB: number;
	    ecoCount: number;
	    prevEcoCount: number;
	    highLoadMin: number;
	    prevHighLoadMin: number;
	    daysObserved: number;
	    series: DayStats[];
	    gameExclusions: string[];
	
	    static createFrom(source: any = {}) {
	        return new WeeklySummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.insufficient = source["insufficient"];
	        this.message = this.convertValues(source["message"], loc.Msg);
	        this.messageRu = source["messageRu"];
	        this.messageEn = source["messageEn"];
	        this.gameHours = source["gameHours"];
	        this.prevGameHours = source["prevGameHours"];
	        this.freezes = source["freezes"];
	        this.prevFreezes = source["prevFreezes"];
	        this.cpuTempPeak = source["cpuTempPeak"];
	        this.prevCPUTempPeak = source["prevCPUTempPeak"];
	        this.gpuTempPeak = source["gpuTempPeak"];
	        this.prevGPUTempPeak = source["prevGPUTempPeak"];
	        this.stableDay = source["stableDay"];
	        this.topCulprit = source["topCulprit"];
	        this.ecoRAMFreedMB = source["ecoRAMFreedMB"];
	        this.prevEcoRAMMB = source["prevEcoRAMMB"];
	        this.ecoCount = source["ecoCount"];
	        this.prevEcoCount = source["prevEcoCount"];
	        this.highLoadMin = source["highLoadMin"];
	        this.prevHighLoadMin = source["prevHighLoadMin"];
	        this.daysObserved = source["daysObserved"];
	        this.series = this.convertValues(source["series"], DayStats);
	        this.gameExclusions = source["gameExclusions"];
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
	export class appPrefs {
	    autoStart: boolean;
	    startMinimized: boolean;
	    closeToTray: boolean;
	    tempAlerts: boolean;
	    showTempOnTray: boolean;
	    trayTipShown: boolean;
	    ecoExitPolicy: string;
	    hiddenGames?: HiddenGamePref[];
	    miniHudPosX: number;
	    miniHudPosY: number;
	    miniHudOpacityPercent: number;
	    miniHudAcrylic: boolean;
	    conflictWatcherEnabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new appPrefs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.autoStart = source["autoStart"];
	        this.startMinimized = source["startMinimized"];
	        this.closeToTray = source["closeToTray"];
	        this.tempAlerts = source["tempAlerts"];
	        this.showTempOnTray = source["showTempOnTray"];
	        this.trayTipShown = source["trayTipShown"];
	        this.ecoExitPolicy = source["ecoExitPolicy"];
	        this.hiddenGames = this.convertValues(source["hiddenGames"], HiddenGamePref);
	        this.miniHudPosX = source["miniHudPosX"];
	        this.miniHudPosY = source["miniHudPosY"];
	        this.miniHudOpacityPercent = source["miniHudOpacityPercent"];
	        this.miniHudAcrylic = source["miniHudAcrylic"];
	        this.conflictWatcherEnabled = source["conflictWatcherEnabled"];
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
	export class ecoHistoryEntry {
	    appName: string;
	    at: time.Time;
	    ramBeforeMB: number;
	    ramAfterMB: number;
	    cpuBefore: number;
	    cpuAfter: number;
	    result: string;
	    sessionId: string;
	    canRevert: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ecoHistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.at = this.convertValues(source["at"], time.Time);
	        this.ramBeforeMB = source["ramBeforeMB"];
	        this.ramAfterMB = source["ramAfterMB"];
	        this.cpuBefore = source["cpuBefore"];
	        this.cpuAfter = source["cpuAfter"];
	        this.result = source["result"];
	        this.sessionId = source["sessionId"];
	        this.canRevert = source["canRevert"];
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

}

export namespace time {
	
	export class Time {
	
	
	    static createFrom(source: any = {}) {
	        return new Time(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

