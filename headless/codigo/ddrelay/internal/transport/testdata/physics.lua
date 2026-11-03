local assets=require("catalog")
local trajectory=require("trajectory")
local map_id, position, direction
local origins, velocities, requests, own_turns, found, cancelled = 0, 0, {}, 0, 0, 0
local job
audit_result=function() return {origins=origins,velocities=velocities,own_turns=own_turns,found=found,cancelled=cancelled} end
local function trunc(n) if n < 0 then return math.ceil(n) end return math.floor(n) end
local function search_turn()
 local state, self, wind = fight.state(), fight.self(), fight.wind()
 if not self or not state.turn or state.turn.actor_id ~= self.id or not wind.current or not self.position then return end
 own_turns = own_turns + 1
 local nearest, distance
 for _, actor in ipairs(fight.actors()) do
  if actor.team ~= self.team and actor.position and actor.hp and actor.hp > 0 then
   local d = math.abs(actor.position.x - self.position.x)
   if not distance or d < distance then nearest, distance = actor, d end
  end
 end
 if not nearest then return end
 local d = nearest.position.x < self.position.x and -1 or 1
 local w = assets.weapon({id=self.weapon_id})
 job = trajectory.search({actor_id=self.id,direction=d,target={x=nearest.position.x,y=nearest.position.y-10,radius=15},force_min=100,force_max=1800,force_step=20,angle_min=math.ceil(w.min_angle),angle_max=math.floor(w.max_angle),angle_step=2,max_steps=300})
end
return {api_version=1,on_packet=function(p)
 if p.code ~= 91 then return end
 local f = p.fields
 if p.direction == 'ServerToClient' then
  if f.subcommand == 103 then map_id = f.map_id; position = nil; job = nil end
  if f.subcommand == 6 then
   local self = fight.self()
   if self then for _, actor in ipairs(f.livings or {}) do if actor.living_id == self.id then position={x=actor.x,y=actor.y} end end end
   search_turn()
  end
  if f.subcommand == 2 then
   for _, projectile in ipairs(f.projectiles) do
    for index, request in ipairs(requests) do
     if projectile.x == request.x and projectile.y == request.y then
      assert(projectile.vx == trunc(request.force * math.cos(math.rad(request.angle))))
      assert(projectile.vy == trunc(request.force * math.sin(math.rad(request.angle))))
      velocities = velocities + 1
      table.remove(requests,index)
      break
     end
    end
   end
  end
 else
  if f.subcommand == 9 then position = {x=f.x,y=f.y}; direction = f.direction end
  if f.subcommand == 7 then direction = f.direction end
  if f.subcommand == 2 then
   local success, slope = pcall(trajectory.slope,{map_id=map_id,x=position.x,y=position.y})
   if not success then console.log("PHYSICS_ERROR",slope.code,slope.message);error(slope) end
   local gun_angle = direction == 1 and (slope - f.angle) or (f.angle - slope + 180)
   local shot = trajectory.launch({x=position.x,y=position.y,direction=direction,slope=slope,gun_angle=gun_angle,force=f.force})
   assert(shot.x == f.x and shot.y == f.y and shot.angle == f.angle and shot.force == f.force)
   origins = origins + 1
   requests[#requests+1] = f
  end
 end
 if job then
  local ok, progress = pcall(function() return job:step(2) end)
  if not ok then
   if progress.code ~= 'trajectory_stale' and progress.code ~= 'trajectory_no_solution' then error(progress) end
   cancelled = cancelled + 1; job = nil
  elseif progress.status == 'found' then
   found = found + 1; job = nil
  end
 end
end,on_close=function()
 console.log('PHYSICS_AUDIT',origins,velocities,own_turns,found,cancelled)
 if origins > 0 then assert(origins == 8 and velocities == 8) end
end}
