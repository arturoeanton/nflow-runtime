function log () {
  profile = get_profile()
  console.log(
   ">>",
    "User:",profile["username"],
    "BoxID:", box_id,
    "BoxName:", box_name, 
    "BoxType:", box_type, 
    "Time:",duration_ms, "ms"
  )
}