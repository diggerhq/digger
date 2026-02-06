import { createServerFn } from "@tanstack/react-start"
import { createUnit, getUnit, listUnits, getUnitVersions, unlockUnit, lockUnit, getUnitStatus, deleteUnit, downloadLatestState, forcePushState, restoreUnitStateVersion } from "./statesman_units"
import { requireAuth } from "./helpers"

export const listUnitsFn = createServerFn({method: 'GET'})
  .handler(async () => {
    const auth = await requireAuth();
    const units : any = await listUnits(auth.organizationId, auth.userId, auth.email);
    return units;
})

export const getUnitFn = createServerFn({method: 'GET'})
  .inputValidator((data : {unitId: string}) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    const unit : any = await getUnit(auth.organizationId, auth.userId, auth.email, data.unitId)
    return unit
})

export const getUnitVersionsFn = createServerFn({method: 'GET'})
  .inputValidator((data : {unitId: string}) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    const unitVersions : any = await getUnitVersions(auth.organizationId, auth.userId, auth.email, data.unitId)
    return unitVersions
})

export const lockUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {unitId: string}) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    const unit : any = await lockUnit(auth.organizationId, auth.userId, auth.email, data.unitId)
    return unit
})

export const unlockUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {unitId: string}) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    const unit : any = await unlockUnit(auth.organizationId, auth.userId, auth.email, data.unitId)
    return unit
})

export const downloadLatestStateFn = createServerFn({method: 'GET'})
  .inputValidator((data : {unitId: string}) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    const state : any = await downloadLatestState(auth.organizationId, auth.userId, auth.email, data.unitId)
    return state
})

export const forcePushStateFn = createServerFn({method: 'POST'})
  .inputValidator((data : {unitId: string, state: string}) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    const state : any = await forcePushState(auth.organizationId, auth.userId, auth.email, data.unitId, data.state)
    return state
})

export const restoreUnitStateVersionFn = createServerFn({method: 'POST'})
  .inputValidator((data : {unitId: string, timestamp: string, lockId: string}) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    const state : any = await restoreUnitStateVersion(auth.organizationId, auth.userId, auth.email, data.unitId, data.timestamp, data.lockId)
    return state
})

export const getUnitStatusFn = createServerFn({method: 'GET'})
  .inputValidator((data : {unitId: string}) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    const unitStatus : any = await getUnitStatus(auth.organizationId, auth.userId, auth.email, data.unitId)
    return unitStatus
})

export const createUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {
    name: string,
    requestId?: string,
    tfeAutoApply?: boolean,
    tfeExecutionMode?: string,
    tfeTerraformVersion?: string,
    tfeEngine?: string,
    tfeWorkingDirectory?: string
  }) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    const unit : any = await createUnit(
      auth.organizationId,
      auth.userId,
      auth.email,
      data.name,
      data.tfeAutoApply,
      data.tfeExecutionMode,
      data.tfeTerraformVersion,
      data.tfeEngine,
      data.tfeWorkingDirectory
    );
    return unit;
})

export const updateUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {
    unitId: string,
    tfeAutoApply?: boolean,
    tfeExecutionMode?: string,
    tfeTerraformVersion?: string,
    tfeEngine?: string,
    tfeWorkingDirectory?: string
  }) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    const { updateUnit } = await import("./statesman_units")
    const unit : any = await updateUnit(
      auth.organizationId,
      auth.userId,
      auth.email,
      data.unitId,
      data.tfeAutoApply,
      data.tfeExecutionMode,
      data.tfeTerraformVersion,
      data.tfeEngine,
      data.tfeWorkingDirectory
    );
    return unit;
})

export const deleteUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {unitId: string}) => data)
  .handler(async ({ data }) => {
    const auth = await requireAuth();
    await deleteUnit(auth.organizationId, auth.userId, auth.email, data.unitId)
})
