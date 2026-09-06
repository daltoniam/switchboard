fn main() -> Result<(), Box<dyn std::error::Error>> {
    std::env::set_var("PROTOC", protoc_bin_vendored::protoc_bin_path()?);
    let proto = proto_path();
    println!("cargo:rerun-if-changed={}", proto.display());
    tonic_build::configure()
        .build_client(true)
        .build_server(false)
        .compile_protos(&[proto], &[proto_include()])?;
    Ok(())
}

fn proto_path() -> std::path::PathBuf {
    if let Ok(path) = std::env::var("SWITCHBOARD_AWM_PROTO") {
        return std::path::PathBuf::from(path);
    }
    let crate_local = manifest_dir().join("proto/awm.proto");
    if crate_local.exists() {
        return crate_local;
    }
    manifest_dir()
        .join("../../api/switchboard/awm/v1/awm.proto")
        .canonicalize()
        .expect("canonical switchboard.awm.v1 proto")
}

fn proto_include() -> std::path::PathBuf {
    let crate_local = manifest_dir().join("proto");
    if crate_local.join("awm.proto").exists() {
        return crate_local;
    }
    manifest_dir()
        .join("../../api")
        .canonicalize()
        .expect("canonical proto include path")
}

fn manifest_dir() -> std::path::PathBuf {
    std::path::PathBuf::from(std::env::var("CARGO_MANIFEST_DIR").expect("CARGO_MANIFEST_DIR"))
}
